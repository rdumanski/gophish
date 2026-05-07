package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAnthropicServer spins up an httptest.NewServer that mimics the
// Anthropic Messages API just enough to exercise the Generator. The
// handler closure receives the request and returns the JSON body to
// send back along with an HTTP status. This keeps each test focused on
// a single failure mode without re-implementing the full API surface.
func fakeAnthropicServer(t *testing.T, handler func(*http.Request) (status int, body string)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := handler(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

// happyResponse encodes a successful Anthropic Messages response whose
// content text is a JSON document matching modelDraft.
func happyResponse(t *testing.T, draft modelDraft, model string) string {
	t.Helper()
	inner, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("encoding modelDraft: %s", err)
	}
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       model,
		"content":     []map[string]interface{}{{"type": "text", "text": string(inner)}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 200},
	}
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("encoding response: %s", err)
	}
	return string(out)
}

func newTestGenerator(t *testing.T, server *httptest.Server) *AnthropicGenerator {
	t.Helper()
	g, err := NewAnthropic(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewAnthropic: %s", err)
	}
	return g
}

func validBrief() Brief {
	return Brief{
		Audience: "IT staff at a finance company",
		Theme:    "password expiration notice",
		Urgency:  UrgencyMedium,
		Length:   LengthShort,
	}
}

func TestNewAnthropicRequiresAPIKey(t *testing.T) {
	if _, err := NewAnthropic(AnthropicConfig{}); err == nil {
		t.Fatal("expected error for empty APIKey, got nil")
	}
}

func TestGenerateHappyPath(t *testing.T) {
	want := modelDraft{
		Subject:  "Password expires in 24 hours",
		TextBody: "Click {{.URL}} to renew, {{.FirstName}}.",
		HTMLBody: `<p>Hi {{.FirstName}}, <a href="{{.URL}}">renew now</a>.</p>{{.Tracker}}`,
		Notes:    "Used short-deadline urgency framing typical of internal IT.",
	}
	server := fakeAnthropicServer(t, func(r *http.Request) (int, string) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("X-Api-Key header missing or wrong: %q", r.Header.Get("X-Api-Key"))
		}
		return http.StatusOK, happyResponse(t, want, "claude-sonnet-4-6")
	})
	g := newTestGenerator(t, server)

	got, err := g.Generate(context.Background(), validBrief())
	if err != nil {
		t.Fatalf("Generate returned error: %s", err)
	}
	if got.Subject != want.Subject || got.Text != want.TextBody || got.HTML != want.HTMLBody {
		t.Errorf("draft mismatch: got %+v want %+v", got, want)
	}
	if got.Notes != want.Notes {
		t.Errorf("notes mismatch: got %q want %q", got.Notes, want.Notes)
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Errorf("Model: got %q want %q", got.Model, "claude-sonnet-4-6")
	}
}

func TestGenerateAppendsMissingVariablesWarning(t *testing.T) {
	// Both bodies miss {{.URL}}; html_body misses {{.Tracker}}.
	draft := modelDraft{
		Subject:  "Reset your password",
		TextBody: "Hi {{.FirstName}}, your password is about to expire.",
		HTMLBody: `<p>Click <a href="https://example.com/reset">here</a>.</p>`,
		Notes:    "Original notes stay intact.",
	}
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, happyResponse(t, draft, "claude-sonnet-4-6")
	})
	g := newTestGenerator(t, server)

	got, err := g.Generate(context.Background(), validBrief())
	if err != nil {
		t.Fatalf("Generate: %s", err)
	}
	if !strings.Contains(got.Notes, "Original notes stay intact.") {
		t.Errorf("Notes lost the model's original commentary: %q", got.Notes)
	}
	if !strings.Contains(got.Notes, "{{.URL}}") || !strings.Contains(got.Notes, "{{.Tracker}}") {
		t.Errorf("Notes missing variable warning: %q", got.Notes)
	}
}

func TestGenerateRefusal(t *testing.T) {
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"content":     []map[string]interface{}{{"type": "text", "text": "I can't help with this request."}},
		"stop_reason": "refusal",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 10},
	}
	body, _ := json.Marshal(resp)
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, string(body)
	})
	g := newTestGenerator(t, server)

	_, err := g.Generate(context.Background(), validBrief())
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("expected ErrRefused, got: %v", err)
	}
}

func TestGenerateUpstream401MapsToProviderAuth(t *testing.T) {
	body := `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusUnauthorized, body
	})
	g := newTestGenerator(t, server)

	_, err := g.Generate(context.Background(), validBrief())
	if !errors.Is(err, errProviderAuth) {
		t.Fatalf("expected errProviderAuth, got: %v", err)
	}
}

func TestGenerateUpstream5xxStaysGeneric(t *testing.T) {
	body := `{"type":"error","error":{"type":"api_error","message":"upstream blew up"}}`
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusInternalServerError, body
	})
	// Disable retries via custom HTTP client + a single-call shortcut: the
	// SDK retries 5xx by default; use option.WithMaxRetries(0) at construction
	// time when we add it. For now, count the requests to confirm at least
	// one happened, then accept whichever final error arrived.
	g, err := NewAnthropic(AnthropicConfig{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewAnthropic: %s", err)
	}
	_, err = g.Generate(context.Background(), validBrief())
	if err == nil {
		t.Fatal("expected error from 500, got nil")
	}
	if errors.Is(err, errProviderAuth) || errors.Is(err, ErrRefused) || errors.Is(err, ErrInvalidBrief) {
		t.Fatalf("5xx misclassified as a typed error: %v", err)
	}
}

func TestGenerateRejectsEmptyBrief(t *testing.T) {
	// Server should never be hit; failing early avoids burning tokens on
	// obviously-malformed input.
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		t.Error("server hit despite empty brief")
		return http.StatusOK, ""
	})
	g := newTestGenerator(t, server)

	_, err := g.Generate(context.Background(), Brief{Audience: "", Theme: "x"})
	if !errors.Is(err, ErrInvalidBrief) {
		t.Fatalf("expected ErrInvalidBrief, got: %v", err)
	}
	_, err = g.Generate(context.Background(), Brief{Audience: "x", Theme: ""})
	if !errors.Is(err, ErrInvalidBrief) {
		t.Fatalf("expected ErrInvalidBrief on empty Theme, got: %v", err)
	}
}

func TestGenerateNonJSONResponseFailsClean(t *testing.T) {
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"content":     []map[string]interface{}{{"type": "text", "text": "I'd love to help, here's a draft: ..."}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 50},
	}
	body, _ := json.Marshal(resp)
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, string(body)
	})
	g := newTestGenerator(t, server)

	_, err := g.Generate(context.Background(), validBrief())
	if err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Fatalf("expected non-JSON error, got: %v", err)
	}
}

func TestGenerateStripsMarkdownFences(t *testing.T) {
	draft := modelDraft{
		Subject:  "S",
		TextBody: "Click {{.URL}} {{.FirstName}}.",
		HTMLBody: `<a href="{{.URL}}">click</a>{{.Tracker}}`,
		Notes:    "n",
	}
	inner, _ := json.Marshal(draft)
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"content":     []map[string]interface{}{{"type": "text", "text": "```json\n" + string(inner) + "\n```"}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 200},
	}
	body, _ := json.Marshal(resp)
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, string(body)
	})
	g := newTestGenerator(t, server)

	got, err := g.Generate(context.Background(), validBrief())
	if err != nil {
		t.Fatalf("Generate: %s", err)
	}
	if got.Subject != "S" {
		t.Errorf("fence-wrapped JSON not parsed: got %+v", got)
	}
}

// happyScoreResponse encodes a successful Anthropic Messages response
// whose content text is a JSON document matching modelScore.
func happyScoreResponse(t *testing.T, score modelScore, model string) string {
	t.Helper()
	inner, err := json.Marshal(score)
	if err != nil {
		t.Fatalf("encoding modelScore: %s", err)
	}
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       model,
		"content":     []map[string]interface{}{{"type": "text", "text": string(inner)}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 200},
	}
	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("encoding response: %s", err)
	}
	return string(out)
}

func validSubject() Subject {
	return Subject{
		Subject: "Your password expires in 24 hours",
		Text:    "Hi {{.FirstName}}, click {{.URL}} to renew.",
		HTML:    `<p>Hi {{.FirstName}}, <a href="{{.URL}}">renew</a>.</p>{{.Tracker}}`,
		From:    "IT Helpdesk",
		Hint:    "IT staff at a finance company",
	}
}

// Compile-time assertion: AnthropicGenerator satisfies Scorer.
var _ Scorer = (*AnthropicGenerator)(nil)

func TestScoreTemplateHappyPath(t *testing.T) {
	want := modelScore{
		Score:           4,
		Rationale:       "Plausible IT-helpdesk pretext, time-bound urgency typical of internal password notices.",
		Strengths:       []string{"Subject line matches typical internal notice", "Urgency framing"},
		Weaknesses:      []string{"Generic greeting could be tighter"},
		WouldMakeHarder: []string{"Personalize from a specific IT staff member", "Reference a real intranet URL pattern"},
	}
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, happyScoreResponse(t, want, "claude-sonnet-4-6")
	})
	g := newTestGenerator(t, server)

	got, err := g.ScoreTemplate(context.Background(), validSubject())
	if err != nil {
		t.Fatalf("ScoreTemplate returned error: %s", err)
	}
	if got.Score != want.Score || got.Rationale != want.Rationale {
		t.Errorf("score mismatch: got %+v want %+v", got, want)
	}
	if len(got.Strengths) != 2 || len(got.Weaknesses) != 1 || len(got.WouldMakeHarder) != 2 {
		t.Errorf("list length mismatch: got %+v", got)
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Errorf("Model: got %q want %q", got.Model, "claude-sonnet-4-6")
	}
}

func TestScoreTemplateRejectsEmptySubject(t *testing.T) {
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		t.Error("server hit despite empty subject")
		return http.StatusOK, ""
	})
	g := newTestGenerator(t, server)

	cases := []Subject{
		{Subject: "", Text: "x", HTML: "<p>x</p>"}, // missing subject line
		{Subject: "Hello", Text: "", HTML: ""},     // missing both bodies
	}
	for _, sub := range cases {
		_, err := g.ScoreTemplate(context.Background(), sub)
		if !errors.Is(err, ErrInvalidSubject) {
			t.Errorf("expected ErrInvalidSubject for %+v, got: %v", sub, err)
		}
	}
}

func TestScoreTemplateRefusal(t *testing.T) {
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"content":     []map[string]interface{}{{"type": "text", "text": "I can't help with this request."}},
		"stop_reason": "refusal",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 10},
	}
	body, _ := json.Marshal(resp)
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, string(body)
	})
	g := newTestGenerator(t, server)

	_, err := g.ScoreTemplate(context.Background(), validSubject())
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("expected ErrRefused, got: %v", err)
	}
}

func TestScoreTemplateUpstream401(t *testing.T) {
	body := `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusUnauthorized, body
	})
	g := newTestGenerator(t, server)

	_, err := g.ScoreTemplate(context.Background(), validSubject())
	if !errors.Is(err, errProviderAuth) {
		t.Fatalf("expected errProviderAuth, got: %v", err)
	}
}

func TestScoreTemplateRejectsOutOfRangeScore(t *testing.T) {
	bad := modelScore{Score: 7, Rationale: "x"}
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, happyScoreResponse(t, bad, "claude-sonnet-4-6")
	})
	g := newTestGenerator(t, server)

	_, err := g.ScoreTemplate(context.Background(), validSubject())
	if err == nil || !strings.Contains(err.Error(), "out-of-range") {
		t.Fatalf("expected out-of-range error, got: %v", err)
	}
}

func TestScoreTemplateNonJSON(t *testing.T) {
	resp := map[string]interface{}{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"content":     []map[string]interface{}{{"type": "text", "text": "Sure, here's my analysis: ..."}},
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 50},
	}
	body, _ := json.Marshal(resp)
	server := fakeAnthropicServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, string(body)
	})
	g := newTestGenerator(t, server)

	_, err := g.ScoreTemplate(context.Background(), validSubject())
	if err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Fatalf("expected non-JSON error, got: %v", err)
	}
}

func TestStripJSONFence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"```json\n{}\n```", "{}"},
		{"```\n{}\n```", "{}"},
		{"  ```json\n{\"a\":1}\n```  ", `{"a":1}`},
	}
	for _, tc := range cases {
		if got := stripJSONFence(tc.in); got != tc.want {
			t.Errorf("stripJSONFence(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
