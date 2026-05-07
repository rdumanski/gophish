package ai

import (
	"context"
	"errors"
)

// Urgency hints at how time-pressured the simulated phish should feel.
type Urgency string

// Urgency values supported by Brief.Urgency.
const (
	UrgencyNone   Urgency = ""       // unset; treated as Medium by the model
	UrgencyLow    Urgency = "low"    // routine reminder
	UrgencyMedium Urgency = "medium" // typical inbox prompt
	UrgencyHigh   Urgency = "high"   // pressing / time-bound
)

// Length is a coarse hint at the length of the generated body.
type Length string

// Length values supported by Brief.Length.
const (
	LengthUnset  Length = ""       // unset; treated as Medium by the model
	LengthShort  Length = "short"  // a few sentences
	LengthMedium Length = "medium" // 1–2 short paragraphs
	LengthLong   Length = "long"   // multiple paragraphs
)

// Brief is the structured input an admin provides when asking the LLM
// to draft a phishing-simulation email template.
//
// Audience and Theme are required; everything else is optional. Empty
// optional fields are filled in by the model based on context.
type Brief struct {
	Audience string  // e.g. "IT staff at a mid-size finance company"
	Theme    string  // e.g. "password expiration notice"
	Urgency  Urgency // optional
	Length   Length  // optional
	Language string  // BCP-47-ish; default "en"
	Brand    string  // optional brand to imitate (e.g. "Microsoft 365")
}

// Draft is the LLM's answer.
//
// Subject, Text, and HTML are populated for use as a Gophish template;
// Notes is a free-form commentary describing the tactics the model
// used (useful for transparency in admin training contexts).
type Draft struct {
	Subject string
	Text    string
	HTML    string
	Notes   string
	Model   string // identifier of the model that produced this draft
}

// Generator drafts a phishing-simulation template from a structured Brief.
//
// Implementations must be safe for concurrent use by multiple goroutines.
// On a model refusal the implementation must return ErrRefused (typically
// wrapped) so callers can surface a 422 rather than a generic 5xx.
type Generator interface {
	Generate(ctx context.Context, brief Brief) (Draft, error)
}

// ErrRefused is returned (typically wrapped) when the underlying model
// declines the request even with the security-training framing. Callers
// should map this to HTTP 422.
var ErrRefused = errors.New("ai: model refused the request")
