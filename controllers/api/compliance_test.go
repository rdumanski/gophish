package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rdumanski/gophish/models"
)

// TestComplianceJSON drives the full /api/compliance/ route (through
// RequireAPIKey, which sets the user context the handler reads) and asserts a
// valid ComplianceReport comes back, with the operator stamped.
func TestComplianceJSON(t *testing.T) {
	ctx := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/compliance/?start=all&api_key="+ctx.apiKey, nil)
	resp := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	report := models.ComplianceReport{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("error decoding ComplianceReport: %v", err)
	}
	if report.Operator != ctx.admin.Username {
		t.Fatalf("expected operator %q, got %q", ctx.admin.Username, report.Operator)
	}
	if report.GeneratedAt.IsZero() {
		t.Fatalf("expected GeneratedAt to be set")
	}
}

// TestCompliancePeriodValidation asserts an inverted range is rejected.
func TestCompliancePeriodValidation(t *testing.T) {
	ctx := setupTest(t)
	req := httptest.NewRequest(http.MethodGet,
		"/api/compliance/?start=2026-06-01&end=2026-01-01&api_key="+ctx.apiKey, nil)
	resp := httptest.NewRecorder()
	ctx.apiServer.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for inverted period, got %d: %s", resp.Code, resp.Body.String())
	}
}
