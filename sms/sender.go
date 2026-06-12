package sms

import (
	"context"
	"errors"
)

// Message is the per-recipient envelope the Sender consumes. To and From
// are E.164 phone numbers; Body is the rendered text, already passed
// through the template context (so {{.URL}} etc. are resolved). The
// dispatcher does not enforce length limits — providers reject messages
// that exceed the carrier-specific cap and that error surfaces back as
// a permanent failure.
type Message struct {
	To   string
	From string
	Body string
}

// Receipt is the provider's acknowledgement that the message was
// accepted for delivery. ProviderID is the upstream identifier (Twilio
// MessageSID, etc.) used later for status-callback reconciliation in
// Phase 8c. Status is the provider's initial state ("queued",
// "accepted", ...) — informational only.
type Receipt struct {
	ProviderID string
	Status     string
}

// Sender is the contract every SMS provider implementation satisfies.
// Implementations must be safe for concurrent use by multiple goroutines
// (the worker batches messages by campaign and may issue several Send
// calls in parallel from the same Sender instance).
//
// On a transient/retryable failure the implementation should return an
// error that is NOT one of the sentinel errors below — the worker treats
// any non-sentinel error as retryable and applies exponential backoff
// via SMSLog.Backoff. Sentinel errors are permanent and route to
// SMSLog.Error which marks the result as failed.
type Sender interface {
	Send(ctx context.Context, m Message) (Receipt, error)
}

// ErrUnregisteredFromNumber is returned (typically wrapped) when the
// provider rejects the From number — e.g. Twilio's 21606 ("not a valid,
// SMS-capable inbound phone number") or 21659 ("not Twilio number").
// Permanent: the operator must fix the SMS profile.
var ErrUnregisteredFromNumber = errors.New("sms: provider rejected the from number")

// ErrInvalidRecipient is returned when the destination number is
// rejected as malformed or unreachable (Twilio 21211 "invalid To").
// Permanent for this recipient; other recipients in the same batch
// continue.
var ErrInvalidRecipient = errors.New("sms: provider rejected the recipient number")

// ErrCarrierBlocked is returned when the carrier filtered the message
// (Twilio 30007 "carrier filtered", 30032 "toll-free not verified",
// 30034 "10DLC not registered"). Permanent: needs operator action
// (registration, unblocking, content change).
var ErrCarrierBlocked = errors.New("sms: carrier blocked the message")

// ErrRefused is returned when the provider refuses the message on
// content/policy grounds. Mirrors ai.ErrRefused. Permanent.
var ErrRefused = errors.New("sms: provider refused the message")

// ErrUnsupportedProvider is returned by NewSender when the requested
// provider string isn't recognised. Caller should surface this as a
// 4xx to the operator (mirrors models.ErrSMSProfileProviderUnsupported
// at the model layer).
var ErrUnsupportedProvider = errors.New("sms: unsupported provider")
