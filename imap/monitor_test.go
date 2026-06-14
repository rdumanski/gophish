package imap

import (
	"testing"

	"github.com/rdumanski/gophish/models"
)

func TestReporterAddress(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"alice@corp.example", "alice@corp.example"},
		{"Alice Example <alice@corp.example>", "alice@corp.example"},
		{"<alice@corp.example>", "alice@corp.example"},
		// Un-encoded UTF-8 display name: mail.ParseAddress fails, regex fallback wins.
		{"Radosław Dumański <rdumanski@gmail.com>", "rdumanski@gmail.com"},
		{"  bob@corp.example  ", "bob@corp.example"},
	}
	for _, tc := range cases {
		if got := reporterAddress(tc.in); got != tc.want {
			t.Errorf("reporterAddress(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPickReporterResult(t *testing.T) {
	// Zero matches: not credited.
	if r, _ := pickReporterResult(nil); r != nil {
		t.Errorf("expected nil for no matches, got %+v", r)
	}
	// Single match: credited, and it's the one we passed (most-recent first).
	one := []models.Result{{RID: "AAA1111"}}
	if r, _ := pickReporterResult(one); r == nil || r.RID != "AAA1111" {
		t.Errorf("expected single match AAA1111, got %+v", r)
	}
	// Multiple matches: ambiguous, not auto-credited.
	many := []models.Result{{RID: "AAA1111"}, {RID: "BBB2222"}}
	if r, reason := pickReporterResult(many); r != nil {
		t.Errorf("expected nil for ambiguous matches, got %+v (%s)", r, reason)
	}
}
