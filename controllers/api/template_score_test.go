package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdumanski/gophish/ai"
)

// mockScorer satisfies both ai.Generator (for srv.aiGenerator) and
// ai.Scorer so the handler's type-assertion succeeds. The Generate
// method is a panic stub — these tests don't exercise it.
type mockScorer struct {
	score ai.Score
	err   error
	calls int
}

func (m *mockScorer) Generate(_ context.Context, _ ai.Brief) (ai.Draft, error) {
	panic("mockScorer.Generate should not be called by score tests")
}

func (m *mockScorer) ScoreTemplate(_ context.Context, _ ai.Subject) (ai.Score, error) {
	m.calls++
	return m.score, m.err
}

// scorerOnlyGenerator is a Generator that does NOT implement Scorer.
// Used to verify the handler returns 503 in that branch.
type scorerOnlyGenerator struct{}

func (scorerOnlyGenerator) Generate(_ context.Context, _ ai.Brief) (ai.Draft, error) {
	panic("not used")
}

func newScoreRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/templates/score", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestScoreTemplate503WhenAIDisabled(t *testing.T) {
	srv := &Server{} // aiGenerator nil
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":"x","text":"y"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestScoreTemplate503WhenProviderDoesntScore(t *testing.T) {
	srv := &Server{aiGenerator: scorerOnlyGenerator{}}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":"x","text":"y"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (provider doesn't implement Scorer), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "scoring") {
		t.Errorf("expected 'scoring' in response body, got: %s", w.Body.String())
	}
}

func TestScoreTemplate200OnHappyPath(t *testing.T) {
	mg := &mockScorer{
		score: ai.Score{
			Score:           4,
			Rationale:       "plausible IT pretext, time-bound urgency",
			Strengths:       []string{"matches typical internal notice"},
			Weaknesses:      []string{"generic greeting"},
			WouldMakeHarder: []string{"reference a real intranet URL"},
			Model:           "claude-sonnet-4-6",
		},
	}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":"Password expiry","text":"hi","html":"<p>hi</p>"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if mg.calls != 1 {
		t.Fatalf("expected scorer called once, got %d", mg.calls)
	}
	resp := decodeBody[scoreTemplateResponse](t, w)
	if resp.Score != 4 || resp.Model != "claude-sonnet-4-6" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if len(resp.Strengths) != 1 || len(resp.Weaknesses) != 1 || len(resp.WouldMakeHarder) != 1 {
		t.Errorf("list lengths off: %+v", resp)
	}
}

func TestScoreTemplate400OnInvalidJSON(t *testing.T) {
	srv := &Server{aiGenerator: &mockScorer{}}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `not json`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestScoreTemplate400OnInvalidSubject(t *testing.T) {
	mg := &mockScorer{err: fmt.Errorf("%w: subject and at least one of text/html are required", ai.ErrInvalidSubject)}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":""}`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from ErrInvalidSubject, got %d", w.Code)
	}
}

func TestScoreTemplate422OnRefusal(t *testing.T) {
	mg := &mockScorer{err: fmt.Errorf("%w: I can't help with this", ai.ErrRefused)}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":"x","text":"y"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestScoreTemplate502OnGenericError(t *testing.T) {
	mg := &mockScorer{err: errors.New("upstream blew up")}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, newScoreRequest(t, `{"subject":"x","text":"y"}`))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestScoreTemplate405OnGET(t *testing.T) {
	srv := &Server{aiGenerator: &mockScorer{}}
	req := httptest.NewRequest(http.MethodGet, "/api/templates/score", nil)
	w := httptest.NewRecorder()
	srv.ScoreTemplate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
