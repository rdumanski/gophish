package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// twilioAPIBase is overridable for tests via the TwilioConfig.BaseURL
// field. Production callers leave it empty.
const twilioAPIBase = "https://api.twilio.com"

// TwilioConfig is the constructor input for NewTwilio.
//
// MessagingServiceSID, when non-empty, takes precedence over From — the
// API accepts either, and using a Messaging Service lets Twilio pick a
// number out of a pool (recommended for any campaign hitting >1
// recipient, since Twilio's anti-spam logic flags single-number bursts).
type TwilioConfig struct {
	AccountSID          string
	AuthToken           string
	From                string // E.164 sender, e.g. "+15005550006"
	MessagingServiceSID string // optional; overrides From
	HTTPClient          *http.Client
	BaseURL             string // override for tests; empty -> twilioAPIBase
}

// TwilioSender is the production implementation of sms.Sender that
// targets Twilio's REST API directly. Stateless aside from the
// constructed credentials; safe for concurrent use.
type TwilioSender struct {
	cfg    TwilioConfig
	client *http.Client
	base   string
}

// NewTwilio constructs a TwilioSender. Returns an error when the
// configuration is missing required fields. Doesn't make network
// calls — credential validation happens on the first Send.
func NewTwilio(cfg TwilioConfig) (*TwilioSender, error) {
	if cfg.AccountSID == "" || cfg.AuthToken == "" {
		return nil, fmt.Errorf("twilio: account_sid and auth_token are required")
	}
	if cfg.From == "" && cfg.MessagingServiceSID == "" {
		return nil, fmt.Errorf("twilio: either from or messaging_service_sid is required")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	base := cfg.BaseURL
	if base == "" {
		base = twilioAPIBase
	}
	return &TwilioSender{cfg: cfg, client: client, base: strings.TrimRight(base, "/")}, nil
}

// twilioResponse covers both the success and error JSON shapes —
// Twilio reuses the same body envelope. On success ErrorCode is 0;
// on failure Status is the HTTP code (mirrored in the body for
// convenience) and ErrorCode is the numeric Twilio code.
type twilioResponse struct {
	SID          string `json:"sid"`
	Status       string `json:"status"`
	ErrorCode    int    `json:"code"`
	ErrorMessage string `json:"message"`
	MoreInfo     string `json:"more_info"`
}

// Send POSTs the message to Twilio's Messages endpoint. Maps known
// Twilio error codes to the package's sentinel errors so the worker
// can decide retry-vs-permanent without parsing the upstream payload.
//
// Reference: https://www.twilio.com/docs/api/errors
func (t *TwilioSender) Send(ctx context.Context, m Message) (Receipt, error) {
	if m.To == "" {
		return Receipt{}, fmt.Errorf("%w: empty to", ErrInvalidRecipient)
	}
	form := url.Values{}
	form.Set("To", m.To)
	if t.cfg.MessagingServiceSID != "" {
		form.Set("MessagingServiceSid", t.cfg.MessagingServiceSID)
	} else {
		// Per Send-call From override (if the caller routes a campaign
		// through a different number) takes precedence over the
		// constructor default. Worker uses the constructor default
		// today; explicit override is here for forward compat.
		from := m.From
		if from == "" {
			from = t.cfg.From
		}
		form.Set("From", from)
	}
	form.Set("Body", m.Body)

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", t.base, url.PathEscape(t.cfg.AccountSID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Receipt{}, fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(t.cfg.AccountSID, t.cfg.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// Network-level failure — let the worker retry.
		return Receipt{}, fmt.Errorf("twilio: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Receipt{}, fmt.Errorf("twilio: read response: %w", err)
	}
	var parsed twilioResponse
	if jerr := json.Unmarshal(body, &parsed); jerr != nil {
		return Receipt{}, fmt.Errorf("twilio: unparseable response (status %d): %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Receipt{ProviderID: parsed.SID, Status: parsed.Status}, nil
	}
	return Receipt{}, mapTwilioError(parsed)
}

// mapTwilioError converts a Twilio numeric code into one of the package
// sentinel errors so the worker can treat it as permanent. Codes not in
// the table are returned as a generic error and treated as retryable.
//
// The taxonomy here is intentionally narrow — only codes that mean
// "this will never succeed without operator action" are mapped to
// permanent. Everything else (rate limits, transient outages, network
// blips) goes through the retry path.
func mapTwilioError(r twilioResponse) error {
	switch r.ErrorCode {
	case 21211, 21217, 21610, 21614:
		// 21211 invalid To, 21217 not a phone number, 21610 unsubscribed
		// recipient, 21614 To is not SMS-capable.
		return fmt.Errorf("%w: twilio %d: %s", ErrInvalidRecipient, r.ErrorCode, r.ErrorMessage)
	case 21606, 21659, 21660, 21663:
		// 21606 not a valid SMS-capable inbound number, 21659 not Twilio
		// number, 21660 number mismatch, 21663 not your number.
		return fmt.Errorf("%w: twilio %d: %s", ErrUnregisteredFromNumber, r.ErrorCode, r.ErrorMessage)
	case 30007, 30032, 30034, 30036:
		// 30007 carrier filtered, 30032 toll-free not verified,
		// 30034 10DLC not registered, 30036 message expired.
		return fmt.Errorf("%w: twilio %d: %s", ErrCarrierBlocked, r.ErrorCode, r.ErrorMessage)
	case 30030, 30450:
		// 30030 message blocked / 30450 unable to deliver — content
		// rejected by carrier policy.
		return fmt.Errorf("%w: twilio %d: %s", ErrRefused, r.ErrorCode, r.ErrorMessage)
	default:
		// Unrecognised code: surface as generic so the worker retries.
		return fmt.Errorf("twilio %d: %s (%s)", r.ErrorCode, r.ErrorMessage, r.MoreInfo)
	}
}
