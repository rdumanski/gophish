package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rdumanski/gophish/models"
)

func TestPhishFilterGETReturnsCurrentRow(t *testing.T) {
	ctx := setupTest(t)
	// Seed via the model so the GET has something non-default to read.
	if err := models.PutPhishFilter(&models.PhishFilter{MinClickSeconds: 8, SandboxIPs: "203.0.113.0/24"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/phish_filter/", nil)
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got models.PhishFilter
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.MinClickSeconds != 8 || got.SandboxIPs != "203.0.113.0/24" {
		t.Errorf("unexpected row: %+v", got)
	}
}

func TestPhishFilterPUTUpsertsAndValidates(t *testing.T) {
	ctx := setupTest(t)

	body := []byte(`{"min_click_seconds":12,"sandbox_ips":"10.0.0.0/8\n192.168.1.1"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/phish_filter/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	// Persisted?
	got, err := models.GetPhishFilter()
	if err != nil {
		t.Fatalf("GetPhishFilter: %v", err)
	}
	if got.MinClickSeconds != 12 || got.SandboxIPs != "10.0.0.0/8\n192.168.1.1" {
		t.Errorf("unexpected persisted row: %+v", got)
	}
}

func TestPhishFilterPUT400OnBadCIDR(t *testing.T) {
	ctx := setupTest(t)

	body := []byte(`{"min_click_seconds":0,"sandbox_ips":"not-an-ip"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/phish_filter/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not-an-ip") {
		t.Errorf("expected the offending entry in the error message, got: %s", w.Body.String())
	}
}

func TestPhishFilterPUT400OnNegativeSeconds(t *testing.T) {
	ctx := setupTest(t)

	body := []byte(`{"min_click_seconds":-3,"sandbox_ips":""}`)
	req := httptest.NewRequest(http.MethodPut, "/api/phish_filter/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPhishFilterPUT400OnInvalidJSON(t *testing.T) {
	ctx := setupTest(t)
	req := httptest.NewRequest(http.MethodPut, "/api/phish_filter/", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPhishFilterMethodNotAllowed(t *testing.T) {
	ctx := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/phish_filter/", nil)
	w := httptest.NewRecorder()
	ctx.apiServer.PhishFilter(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
