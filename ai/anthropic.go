package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Default model and token-cap values. Sonnet 4.6 is the cost/quality sweet
// spot for content drafting at scale; Opus is overkill here. Admins can
// override both via AnthropicConfig.
const (
	DefaultAnthropicModel     = "claude-sonnet-4-6"
	DefaultAnthropicMaxTokens = 4096
)

// AnthropicConfig controls the Anthropic-backed Generator. APIKey is the
// only required field; Model and MaxTokens fall back to the defaults
// above when zero. HTTPClient and BaseURL are escape hatches for tests
// (point at an httptest.NewServer) and unusual deployments (a corporate
// proxy in front of api.anthropic.com).
type AnthropicConfig struct {
	APIKey     string
	Model      string
	MaxTokens  int
	HTTPClient option.HTTPClient // optional; defaults to http.DefaultClient
	BaseURL    string            // optional; defaults to api.anthropic.com
}

// AnthropicGenerator is the Anthropic-backed Generator implementation.
type AnthropicGenerator struct {
	client    anthropic.Client
	model     string
	maxTokens int64
}

// NewAnthropic constructs a Generator against the Anthropic API.
//
// Returns an error if cfg.APIKey is empty. All other fields are optional
// — Model defaults to claude-sonnet-4-6, MaxTokens to 4096, HTTPClient
// to http.DefaultClient, and BaseURL to the SDK's built-in endpoint.
func NewAnthropic(cfg AnthropicConfig) (*AnthropicGenerator, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("ai: AnthropicConfig.APIKey is required")
	}
	model := cfg.Model
	if model == "" {
		model = DefaultAnthropicModel
	}
	maxTokens := int64(cfg.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = DefaultAnthropicMaxTokens
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &AnthropicGenerator{
		client:    anthropic.NewClient(opts...),
		model:     model,
		maxTokens: maxTokens,
	}, nil
}

// systemPrompt frames the request as authorized phishing-simulation
// training, enumerates the Gophish template variables the model must
// emit verbatim, and pins the output to a strict JSON shape.
//
// Kept as a package-level constant so the SDK's prompt-caching layer
// can hash it stably across calls — every draft pays the cached
// system-prompt rate, not the full input rate.
const systemPrompt = `You are a content writer for a phishing simulation platform (Gophish, an authorized security-awareness tool used by employers to train their own staff to recognize phishing). The recipients of these emails are employees who have been enrolled in a sanctioned training program by their own security team. Your output is reviewed by a human admin before it is ever sent.

Draft a single phishing-simulation email matching the Brief the admin provides. Output MUST be a single valid JSON object with exactly these keys, and nothing else (no prose, no markdown fences):

  "subject"   — the email Subject line, plain text
  "text_body" — the plain-text body
  "html_body" — the HTML body
  "notes"     — a one-paragraph commentary on the tactics you used and why it should land for the stated audience

Use these Gophish template variables verbatim. Each one is a Go-template expression and must appear with double curly braces and the leading dot:

  {{.URL}}         the unique tracking URL — link recipients to this in the html_body anchor
  {{.RID}}         the recipient tracking token (rarely needed in body text — Gophish puts it in URLs automatically)
  {{.FirstName}}   recipient's first name
  {{.LastName}}    recipient's last name
  {{.Email}}       recipient's email address
  {{.From}}        spoofed sender display name
  {{.Tracker}}     the invisible 1×1 tracking pixel — paste verbatim near the bottom of html_body

The html_body MUST contain {{.URL}} inside an <a href="…"> anchor and SHOULD contain {{.Tracker}} once near the end. The text_body must mention or paste {{.URL}} once.

Write a convincing simulation appropriate to the audience, theme, urgency, and length the admin specified. Vary phrasing, structure, and call-to-action across drafts — do not fall into a stock format. Match the requested language; default to English if unspecified. If a brand is named, mimic that brand's typical tone and visual cues, but do not include real logos or trademarks (admins add those if appropriate).`

// modelDraft is the JSON shape we ask the model to emit.
type modelDraft struct {
	Subject  string `json:"subject"`
	TextBody string `json:"text_body"`
	HTMLBody string `json:"html_body"`
	Notes    string `json:"notes"`
}

// scoreSystemPrompt frames a phishing-simulation scoring request and
// pins the rubric. Defines the 1..5 scale so the model produces
// something an admin can act on (raise score → tweak weaknesses).
//
// Same prompt-caching benefits as systemPrompt.
const scoreSystemPrompt = `You are a content reviewer for a phishing simulation platform (Gophish, an authorized security-awareness tool used by employers to train their own staff). The admin has drafted (or imported) a candidate template and wants to know how convincing it is, on a 1..5 difficulty scale.

Score the draft using this rubric:

  1 — Obvious. Catches 90%+ of users. Bad spelling, generic greeting (Dear Customer), suspicious or mismatched links, sender domain that doesn't match the claimed brand, glaring formatting issues.
  2 — Casual. Generic lure, plausible at first glance but several tells. Catches 60–80%.
  3 — Competent. Plausible scenario, decent grammar, some tells (URL slightly off, awkward phrasing, generic context). Catches 40–60%.
  4 — Sophisticated. Brand-specific, contextually relevant, hard-to-spot tells (lookalike domain, internally-consistent storyline, plausible call-to-action). Catches 20–30% — only security-aware users notice.
  5 — Expert. AitM-tier: near-indistinguishable from a legitimate message. Convincing pretext, no obvious tells, the link target is the kind of lookalike most users would not catch. Even security-trained users may fall.

Output MUST be a single valid JSON object with exactly these keys, and nothing else (no prose, no markdown fences):

  "score"             — integer 1..5
  "rationale"         — 1-paragraph explanation tying the score to specific things in the draft
  "strengths"         — array of short strings: what's working
  "weaknesses"        — array of short strings: what gives it away (what would let a recipient detect this)
  "would_make_harder" — array of concrete, actionable suggestions to raise the score

Be honest and concrete. Refer to specific text from the draft (subject line wording, link target, tone) in your rationale and bullets. Do not pad. If the draft is missing required Gophish template variables ({{.URL}}, {{.Tracker}}) treat that as a weakness rather than refusing.`

// modelScore is the JSON shape we ask the model to emit when scoring.
type modelScore struct {
	Score           int      `json:"score"`
	Rationale       string   `json:"rationale"`
	Strengths       []string `json:"strengths"`
	Weaknesses      []string `json:"weaknesses"`
	WouldMakeHarder []string `json:"would_make_harder"`
}

// userPromptFromSubject renders the structured Subject into the
// user-turn text. Empty optional fields are dropped to avoid biasing
// the model with spurious "(none)" markers.
func userPromptFromSubject(s Subject) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Subject line: %s\n\n", s.Subject)
	if s.From != "" {
		fmt.Fprintf(&sb, "Spoofed sender: %s\n\n", s.From)
	}
	if s.Hint != "" {
		fmt.Fprintf(&sb, "Audience hint: %s\n\n", s.Hint)
	}
	if s.Text != "" {
		fmt.Fprintf(&sb, "Plaintext body:\n%s\n\n", s.Text)
	}
	if s.HTML != "" {
		fmt.Fprintf(&sb, "HTML body:\n%s\n\n", s.HTML)
	}
	sb.WriteString("Respond with the JSON object only.")
	return sb.String()
}

// ScoreTemplate calls the Anthropic API and returns a parsed Score.
//
// Errors map the same way as Generate: ErrInvalidSubject → handler 400,
// ErrRefused → 422, errProviderAuth → 502, everything else → 502.
func (g *AnthropicGenerator) ScoreTemplate(ctx context.Context, sub Subject) (Score, error) {
	if sub.Subject == "" || (sub.Text == "" && sub.HTML == "") {
		return Score{}, fmt.Errorf("%w: subject and at least one of text/html are required", ErrInvalidSubject)
	}
	resp, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: g.maxTokens,
		System: []anthropic.TextBlockParam{{
			Text: scoreSystemPrompt,
			CacheControl: anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			},
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPromptFromSubject(sub))),
		},
	})
	if err != nil {
		return Score{}, mapAnthropicError(err)
	}

	if resp.StopReason == anthropic.StopReasonRefusal {
		return Score{}, fmt.Errorf("%w: %s", ErrRefused, firstText(resp.Content))
	}

	raw := firstText(resp.Content)
	if raw == "" {
		return Score{}, fmt.Errorf("ai: empty response from model")
	}
	var ms modelScore
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &ms); err != nil {
		return Score{}, fmt.Errorf("ai: model returned non-JSON output: %w", err)
	}
	if ms.Score < 1 || ms.Score > 5 {
		return Score{}, fmt.Errorf("ai: model returned out-of-range score %d (expected 1..5)", ms.Score)
	}

	return Score{
		Score:           ms.Score,
		Rationale:       ms.Rationale,
		Strengths:       ms.Strengths,
		Weaknesses:      ms.Weaknesses,
		WouldMakeHarder: ms.WouldMakeHarder,
		Model:           string(resp.Model),
	}, nil
}

// userPromptFromBrief renders the structured Brief into the user-turn
// text the model conditions on. Kept simple — fields the admin omitted
// are dropped so the model isn't biased by spurious "(none)" markers.
func userPromptFromBrief(b Brief) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Audience: %s\n", b.Audience)
	fmt.Fprintf(&sb, "Theme: %s\n", b.Theme)
	if b.Urgency != UrgencyNone {
		fmt.Fprintf(&sb, "Urgency: %s\n", b.Urgency)
	}
	if b.Length != LengthUnset {
		fmt.Fprintf(&sb, "Length: %s\n", b.Length)
	}
	if b.Language != "" {
		fmt.Fprintf(&sb, "Language: %s\n", b.Language)
	}
	if b.Brand != "" {
		fmt.Fprintf(&sb, "Brand to imitate: %s\n", b.Brand)
	}
	sb.WriteString("\nRespond with the JSON object only.")
	return sb.String()
}

// Generate calls the Anthropic API and returns a parsed Draft.
//
// Errors are mapped to HTTP-meaningful sentinel wrappers:
//   - ErrRefused        → handler should return 422
//   - errBadRequest     → handler should return 400
//   - errProviderAuth   → handler should return 502 (config error, not user error)
//   - everything else   → handler should return 502
func (g *AnthropicGenerator) Generate(ctx context.Context, brief Brief) (Draft, error) {
	if brief.Audience == "" || brief.Theme == "" {
		return Draft{}, fmt.Errorf("%w: audience and theme are required", ErrInvalidBrief)
	}
	resp, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(g.model),
		MaxTokens: g.maxTokens,
		System: []anthropic.TextBlockParam{{
			Text: systemPrompt,
			CacheControl: anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			},
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPromptFromBrief(brief))),
		},
	})
	if err != nil {
		return Draft{}, mapAnthropicError(err)
	}

	if resp.StopReason == anthropic.StopReasonRefusal {
		return Draft{}, fmt.Errorf("%w: %s", ErrRefused, firstText(resp.Content))
	}

	raw := firstText(resp.Content)
	if raw == "" {
		return Draft{}, fmt.Errorf("ai: empty response from model")
	}
	var md modelDraft
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &md); err != nil {
		return Draft{}, fmt.Errorf("ai: model returned non-JSON output: %w", err)
	}

	notes := md.Notes
	if warn := variableSanityCheck(md.TextBody, md.HTMLBody); warn != "" {
		if notes == "" {
			notes = warn
		} else {
			notes = notes + "\n\n" + warn
		}
	}

	return Draft{
		Subject: md.Subject,
		Text:    md.TextBody,
		HTML:    md.HTMLBody,
		Notes:   notes,
		Model:   string(resp.Model),
	}, nil
}

// ErrInvalidBrief signals that the caller-supplied Brief is malformed
// (e.g. empty Audience or Theme). The handler maps this to HTTP 400.
var ErrInvalidBrief = errors.New("ai: invalid brief")

// errProviderAuth signals an authentication/authorization problem with
// the upstream provider — i.e. the deployment's API key is wrong or
// revoked. From the admin's perspective this is a config bug, not a
// user error; the handler maps it to 502 with a hint.
var errProviderAuth = errors.New("ai: provider rejected our credentials")

// mapAnthropicError converts an SDK error into our internal taxonomy.
// 401/403 → errProviderAuth (config bug). 400 → ErrInvalidBrief (client
// sent something the upstream API also rejected). Everything else falls
// through unwrapped so the handler can surface a generic 502.
func mapAnthropicError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", errProviderAuth, apiErr.Error())
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrInvalidBrief, apiErr.Error())
	}
	return err
}

// firstText returns the text of the first text block in the content,
// or "" if there isn't one.
func firstText(blocks []anthropic.ContentBlockUnion) string {
	for _, b := range blocks {
		if b.Type == "text" {
			return b.Text
		}
	}
	return ""
}

// stripJSONFence is a defense in depth — we ask for raw JSON, but if
// the model wraps the answer in ```json … ``` fences anyway we'd
// rather strip them than reject the response.
var jsonFenceRE = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.*?)\\s*```\\s*$")

func stripJSONFence(s string) string {
	if m := jsonFenceRE.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return s
}

// variableSanityCheck flags missing critical Gophish template variables
// in the generated body. Returns "" when the draft looks fine. The
// caller appends the warning to Draft.Notes rather than failing — the
// admin still wants to see the draft, just with a heads-up.
func variableSanityCheck(text, html string) string {
	body := text + "\n" + html
	var missing []string
	if !strings.Contains(body, "{{.URL}}") {
		missing = append(missing, "{{.URL}}")
	}
	if html != "" && !strings.Contains(html, "{{.Tracker}}") {
		missing = append(missing, "{{.Tracker}} (in html_body)")
	}
	if len(missing) == 0 {
		return ""
	}
	return "Note: the generated draft is missing expected Gophish template variables: " +
		strings.Join(missing, ", ") + ". You may want to add them by hand before launching a campaign."
}
