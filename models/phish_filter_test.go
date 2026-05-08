package models

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rdumanski/gophish/config"
	"gopkg.in/check.v1"
)

// --- Get / Put round-trip ----------------------------------------------------

func (s *ModelsSuite) TestGetPhishFilterCreatesEmptyRow(c *check.C) {
	pf, err := GetPhishFilter()
	c.Assert(err, check.IsNil)
	c.Assert(pf.ID, check.Equals, int64(1))
	c.Assert(pf.MinClickSeconds, check.Equals, 0)
	c.Assert(pf.SandboxIPs, check.Equals, "")
}

func (s *ModelsSuite) TestPutPhishFilterRoundtrip(c *check.C) {
	pf := PhishFilter{
		MinClickSeconds: 7,
		SandboxIPs:      "10.0.0.0/8\n192.168.1.5",
	}
	c.Assert(PutPhishFilter(&pf), check.IsNil)

	got, err := GetPhishFilter()
	c.Assert(err, check.IsNil)
	c.Assert(got.MinClickSeconds, check.Equals, 7)
	c.Assert(got.SandboxIPs, check.Equals, "10.0.0.0/8\n192.168.1.5")
	c.Assert(got.UpdatedAt.IsZero(), check.Equals, false)
}

func (s *ModelsSuite) TestPutPhishFilterRejectsNegativeSeconds(c *check.C) {
	pf := PhishFilter{MinClickSeconds: -1}
	err := PutPhishFilter(&pf)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "min_click_seconds must be >= 0")
}

func (s *ModelsSuite) TestPutPhishFilterRejectsBadCIDR(c *check.C) {
	pf := PhishFilter{SandboxIPs: "10.0.0.0/8\nnot-an-ip\n"}
	err := PutPhishFilter(&pf)
	c.Assert(err, check.NotNil)
	c.Assert(errors.Is(err, ErrInvalidSandboxIP), check.Equals, true)
}

// --- Seed-from-config --------------------------------------------------------

func (s *ModelsSuite) TestSeedPhishFilterFromConfigSeedsEmptyRow(c *check.C) {
	cfg := config.PhishFilterConfig{
		MinClickSeconds: 5,
		SandboxIPs:      []string{"203.0.113.0/24"},
	}
	c.Assert(SeedPhishFilterFromConfig(cfg), check.IsNil)

	got, err := GetPhishFilter()
	c.Assert(err, check.IsNil)
	c.Assert(got.MinClickSeconds, check.Equals, 5)
	c.Assert(got.SandboxIPs, check.Equals, "203.0.113.0/24")
}

func (s *ModelsSuite) TestSeedPhishFilterFromConfigIsIdempotent(c *check.C) {
	// Manually populate the row first.
	pf := PhishFilter{MinClickSeconds: 9, SandboxIPs: "10.10.10.10/32"}
	c.Assert(PutPhishFilter(&pf), check.IsNil)

	cfg := config.PhishFilterConfig{
		MinClickSeconds: 99,
		SandboxIPs:      []string{"203.0.113.0/24"},
	}
	c.Assert(SeedPhishFilterFromConfig(cfg), check.IsNil)

	// DB row is the source of truth — config values must NOT clobber it.
	got, err := GetPhishFilter()
	c.Assert(err, check.IsNil)
	c.Assert(got.MinClickSeconds, check.Equals, 9)
	c.Assert(got.SandboxIPs, check.Equals, "10.10.10.10/32")
}

func (s *ModelsSuite) TestSeedPhishFilterFromConfigEmptyConfigIsNoop(c *check.C) {
	cfg := config.PhishFilterConfig{} // zero values
	c.Assert(SeedPhishFilterFromConfig(cfg), check.IsNil)

	// No row should have been created.
	var count int64
	c.Assert(db.Model(&PhishFilter{}).Count(&count).Error, check.IsNil)
	c.Assert(count, check.Equals, int64(0))
}

// --- Matcher (relocated from controllers/phish_test.go) ----------------------

func TestBuildMatcherEmptyList(t *testing.T) {
	m, err := buildMatcher("", 0)
	if err != nil {
		t.Fatalf("buildMatcher empty: %v", err)
	}
	if m == nil {
		t.Fatalf("expected non-nil matcher")
	}
	if got, want := m.minClickDuration, time.Duration(0); got != want {
		t.Errorf("minClickDuration = %v, want %v", got, want)
	}
	if len(m.networks) != 0 {
		t.Errorf("expected zero networks, got %d", len(m.networks))
	}
}

func TestBuildMatcherBareIPv4PromotedToSlash32(t *testing.T) {
	m, err := buildMatcher("10.1.2.3\n", 5)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if len(m.networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(m.networks))
	}
	if m.networks[0].String() != "10.1.2.3/32" {
		t.Errorf("expected /32 promotion, got %s", m.networks[0].String())
	}
}

func TestBuildMatcherCIDRsAndBlankLines(t *testing.T) {
	in := "10.0.0.0/8\n\n  \n192.168.0.0/16\n"
	m, err := buildMatcher(in, 0)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if len(m.networks) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(m.networks))
	}
}

func TestBuildMatcherRejectsGarbage(t *testing.T) {
	_, err := buildMatcher("not-an-ip", 0)
	if err == nil || !errors.Is(err, ErrInvalidSandboxIP) {
		t.Fatalf("expected ErrInvalidSandboxIP, got %v", err)
	}
}

func TestClassifyMinClickSeconds(t *testing.T) {
	m, err := buildMatcher("", 10)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	send := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tooFast := send.Add(3 * time.Second)
	humanSpeed := send.Add(30 * time.Second)

	if filtered, _ := m.classify(tooFast, send, ""); !filtered {
		t.Errorf("3s click should be filtered as sandbox under min=10s")
	}
	if filtered, _ := m.classify(humanSpeed, send, ""); filtered {
		t.Errorf("30s click should NOT be filtered under min=10s")
	}
}

func TestClassifyZeroSendDateSkipsTimeFilter(t *testing.T) {
	m, err := buildMatcher("", 10)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if filtered, _ := m.classify(time.Now(), time.Time{}, ""); filtered {
		t.Errorf("zero sendDate should disable the time filter to avoid false positives")
	}
}

func TestClassifyIPCIDRMatch(t *testing.T) {
	m, err := buildMatcher("203.0.113.0/24", 0)
	if err != nil {
		t.Fatalf("buildMatcher: %v", err)
	}
	if filtered, reason := m.classify(time.Now(), time.Now(), "203.0.113.45"); !filtered {
		t.Errorf("expected sandbox match, got reason=%q", reason)
	}
	if filtered, _ := m.classify(time.Now(), time.Now(), "198.51.100.1"); filtered {
		t.Errorf("198.51.100.1 should not match 203.0.113.0/24")
	}
}

func TestClassifyNilMatcherIsFilterOff(t *testing.T) {
	var m *sandboxMatcher
	if filtered, _ := m.classify(time.Now(), time.Now(), "203.0.113.45"); filtered {
		t.Errorf("nil matcher should never filter")
	}
}

// --- Filtered helper ---------------------------------------------------------

func (s *ModelsSuite) TestFilteredHelperHonorsCurrentPolicy(c *check.C) {
	// Set policy: 5 second floor, no IP list.
	c.Assert(PutPhishFilter(&PhishFilter{MinClickSeconds: 5}), check.IsNil)

	send := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	fast := Event{Time: send.Add(2 * time.Second)}
	slow := Event{Time: send.Add(60 * time.Second)}

	if filtered, _ := Filtered(fast, send); !filtered {
		c.Errorf("fast click should be filtered")
	}
	if filtered, _ := Filtered(slow, send); filtered {
		c.Errorf("slow click should not be filtered")
	}
}

func (s *ModelsSuite) TestFilteredHelperPolicyChangePropagates(c *check.C) {
	// Phase 7c.2's defining property: changing the policy in DB
	// retroactively reclassifies events at query time.
	send := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	e := Event{Time: send.Add(7 * time.Second)}

	// Initial policy: 5s floor — 7s click is allowed through.
	c.Assert(PutPhishFilter(&PhishFilter{MinClickSeconds: 5}), check.IsNil)
	if filtered, _ := Filtered(e, send); filtered {
		c.Errorf("under min=5s, 7s click should not be filtered")
	}

	// Tighten to 10s — same event now classified as sandbox.
	c.Assert(PutPhishFilter(&PhishFilter{MinClickSeconds: 10}), check.IsNil)
	if filtered, _ := Filtered(e, send); !filtered {
		c.Errorf("under min=10s, 7s click should be filtered")
	}
}

func (s *ModelsSuite) TestFilteredHelperUsesIPFromEventDetails(c *check.C) {
	c.Assert(PutPhishFilter(&PhishFilter{SandboxIPs: "203.0.113.0/24"}), check.IsNil)

	d := EventDetails{Browser: map[string]string{"address": "203.0.113.7"}}
	body, err := json.Marshal(d)
	c.Assert(err, check.IsNil)
	e := Event{Time: time.Now(), Details: string(body)}

	filtered, reason := Filtered(e, time.Now())
	c.Assert(filtered, check.Equals, true)
	c.Assert(reason, check.Equals, "sandbox_ip:203.0.113.0/24")
}

func (s *ModelsSuite) TestFilteredHelperEmptyDetailsIsSafe(c *check.C) {
	c.Assert(PutPhishFilter(&PhishFilter{SandboxIPs: "203.0.113.0/24"}), check.IsNil)
	e := Event{Time: time.Now()} // no Details
	filtered, _ := Filtered(e, time.Now())
	c.Assert(filtered, check.Equals, false)
}
