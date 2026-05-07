package ai

import (
	"context"
	"errors"
)

// Subject is the structured input to Scorer.ScoreTemplate. At minimum
// the candidate template's Subject and at least one of Text or HTML
// must be non-empty; everything else is optional context that helps
// the model reason about the audience.
type Subject struct {
	Subject string // template subject line
	Text    string // plaintext body
	HTML    string // HTML body
	From    string // optional spoofed sender display name
	Hint    string // optional audience hint, e.g. "IT staff at a bank"
}

// Score is the LLM's evaluation of a candidate phishing-simulation
// template, on a 1..5 difficulty scale defined in the system prompt.
//
// Higher Score = harder for users to detect (more sophisticated lure).
// Strengths and Weaknesses are short bullet items the model identified;
// WouldMakeHarder lists concrete tweaks to raise the score.
type Score struct {
	Score           int      // 1..5
	Rationale       string   // 1-paragraph explanation
	Strengths       []string // what's working
	Weaknesses      []string // what gives it away
	WouldMakeHarder []string // concrete suggestions to raise the score
	Model           string   // identifier of the model that produced this score
}

// Scorer evaluates a candidate phishing-simulation template and returns
// a difficulty Score. Implementations must be safe for concurrent use
// by multiple goroutines.
//
// On a model refusal the implementation must return ErrRefused
// (typically wrapped) so the handler can surface a 422 rather than a
// generic 5xx. On caller-side validation failures it returns
// ErrInvalidSubject.
type Scorer interface {
	ScoreTemplate(ctx context.Context, sub Subject) (Score, error)
}

// ErrInvalidSubject signals that the caller-supplied Subject is
// missing required fields (Subject is empty, or both Text and HTML are
// empty). The handler maps this to HTTP 400.
var ErrInvalidSubject = errors.New("ai: invalid subject for scoring")
