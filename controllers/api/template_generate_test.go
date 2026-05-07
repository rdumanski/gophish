package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdumanski/gophish/ai"
)

// mockGenerator is a stand-in ai.Generator that lets each test pin the
// (Draft, error) it wants the handler to see.
type mockGenerator struct {
	draft ai.Draft
	err   error
	calls int
}

func (m *mockGenerator) Generate(_ context.Context, _ ai.Brief) (ai.Draft, error) {
	m.calls++
	return m.draft, m.err
}

func newGenerateRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/templates/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response body: %s", err)
	}
	return out
}

func TestGenerateTemplate503WhenAIDisabled(t *testing.T) {
	srv := &Server{} // aiGenerator is nil — exactly what we want
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `{"audience":"x","theme":"y"}`))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGenerateTemplate200OnHappyPath(t *testing.T) {
	mg := &mockGenerator{
		draft: ai.Draft{
			Subject: "Test subject",
			Text:    "plain {{.URL}}",
			HTML:    `<a href="{{.URL}}">click</a>{{.Tracker}}`,
			Notes:   "tactics here",
			Model:   "claude-sonnet-4-6",
		},
	}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `{"audience":"IT","theme":"password expiry","urgency":"medium","length":"short"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if mg.calls != 1 {
		t.Fatalf("expected generator called once, got %d", mg.calls)
	}
	resp := decodeBody[generateTemplateResponse](t, w)
	if resp.Subject != "Test subject" || resp.Model != "claude-sonnet-4-6" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGenerateTemplate400OnInvalidJSON(t *testing.T) {
	srv := &Server{aiGenerator: &mockGenerator{}}
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `not valid json`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGenerateTemplate400OnInvalidBrief(t *testing.T) {
	mg := &mockGenerator{err: fmt.Errorf("%w: audience and theme are required", ai.ErrInvalidBrief)}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `{"audience":"","theme":""}`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from ErrInvalidBrief, got %d", w.Code)
	}
}

func TestGenerateTemplate422OnRefusal(t *testing.T) {
	mg := &mockGenerator{err: fmt.Errorf("%w: I can't help with this", ai.ErrRefused)}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `{"audience":"x","theme":"y"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 from ErrRefused, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGenerateTemplate502OnGenericProviderError(t *testing.T) {
	mg := &mockGenerator{err: errors.New("upstream blew up")}
	srv := &Server{aiGenerator: mg}
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, newGenerateRequest(t, `{"audience":"x","theme":"y"}`))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 from generic error, got %d", w.Code)
	}
}

func TestGenerateTemplate405OnGET(t *testing.T) {
	srv := &Server{aiGenerator: &mockGenerator{}}
	req := httptest.NewRequest(http.MethodGet, "/api/templates/generate", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	srv.GenerateTemplate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
