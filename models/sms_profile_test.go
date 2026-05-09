package models

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/check.v1"
)

// --- ValidatePhone (E.164) ---------------------------------------------------

func TestValidatePhoneAccepts(t *testing.T) {
	cases := []string{
		"+15555551234",   // US
		"+442071234567",  // UK
		"+33123456789",   // FR
		"+12025550100",   // US gov pool
		"+8613800138000", // CN
	}
	for _, s := range cases {
		if err := ValidatePhone(s); err != nil {
			t.Errorf("ValidatePhone(%q) = %v, want nil", s, err)
		}
	}
}

func TestValidatePhoneRejects(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"missing plus":    "15555551234",
		"leading zero":    "+05555551234",
		"too short":       "+1",
		"too long":        "+1234567890123456",
		"letters":         "+1555ABC1234",
		"spaces":          "+1 555 555 1234",
		"hyphens":         "+1-555-555-1234",
		"local format":    "(555) 555-1234",
		"international 0": "+0555551234",
	}
	for label, s := range cases {
		if err := ValidatePhone(s); err == nil {
			t.Errorf("ValidatePhone(%q) [%s] = nil, want ErrInvalidPhone", s, label)
		} else if !errors.Is(err, ErrInvalidPhone) {
			t.Errorf("ValidatePhone(%q) [%s] = %v, want ErrInvalidPhone", s, label, err)
		}
	}
}

// --- SMSProfile.Validate -----------------------------------------------------

func TestSMSProfileValidateNameRequired(t *testing.T) {
	p := SMSProfile{Provider: "twilio", AccountSID: "AC1", AuthToken: "tok", FromNumber: "+15555551234"}
	if err := p.Validate(); !errors.Is(err, ErrSMSProfileNameNotSpecified) {
		t.Errorf("expected ErrSMSProfileNameNotSpecified, got %v", err)
	}
}

func TestSMSProfileValidateProviderDefaultsToTwilio(t *testing.T) {
	p := SMSProfile{Name: "p", AccountSID: "AC1", AuthToken: "tok", FromNumber: "+15555551234"}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if p.Provider != "twilio" {
		t.Errorf("Provider = %q, want twilio", p.Provider)
	}
}

func TestSMSProfileValidateRejectsUnknownProvider(t *testing.T) {
	p := SMSProfile{Name: "p", Provider: "bogus", AccountSID: "AC1", AuthToken: "tok", FromNumber: "+15555551234"}
	err := p.Validate()
	if !errors.Is(err, ErrSMSProfileProviderUnsupported) {
		t.Errorf("expected ErrSMSProfileProviderUnsupported, got %v", err)
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should name the offending provider, got %v", err)
	}
}

func TestSMSProfileValidateRequiresCredentials(t *testing.T) {
	p := SMSProfile{Name: "p", Provider: "twilio", AuthToken: "tok", FromNumber: "+15555551234"}
	if err := p.Validate(); !errors.Is(err, ErrSMSProfileMissingCredentials) {
		t.Errorf("expected ErrSMSProfileMissingCredentials (no SID), got %v", err)
	}
	p = SMSProfile{Name: "p", Provider: "twilio", AccountSID: "AC1", FromNumber: "+15555551234"}
	if err := p.Validate(); !errors.Is(err, ErrSMSProfileMissingCredentials) {
		t.Errorf("expected ErrSMSProfileMissingCredentials (no token), got %v", err)
	}
}

func TestSMSProfileValidateRequiresFromOrMessagingService(t *testing.T) {
	p := SMSProfile{Name: "p", Provider: "twilio", AccountSID: "AC1", AuthToken: "tok"}
	if err := p.Validate(); !errors.Is(err, ErrSMSProfileMissingFromNumber) {
		t.Errorf("expected ErrSMSProfileMissingFromNumber, got %v", err)
	}
	// Messaging Service SID alone is sufficient.
	p.MessagingServiceSID = "MGxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	if err := p.Validate(); err != nil {
		t.Errorf("with MessagingServiceSID set: validate = %v, want nil", err)
	}
}

func TestSMSProfileValidateRejectsBadFromNumber(t *testing.T) {
	p := SMSProfile{Name: "p", Provider: "twilio", AccountSID: "AC1", AuthToken: "tok", FromNumber: "555-555-1234"}
	err := p.Validate()
	if !errors.Is(err, ErrInvalidPhone) {
		t.Errorf("expected ErrInvalidPhone, got %v", err)
	}
}

// --- DB round-trip via gocheck suite ----------------------------------------

func (s *ModelsSuite) TestSMSProfilePostAndGet(c *check.C) {
	p := SMSProfile{
		UserID:     1,
		Name:       "Twilio Sandbox",
		Provider:   "twilio",
		AccountSID: "ACdeadbeef",
		AuthToken:  "tok",
		FromNumber: "+15005550006",
	}
	c.Assert(PostSMSProfile(&p), check.IsNil)
	c.Assert(p.Id > 0, check.Equals, true)

	got, err := GetSMSProfile(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Name, check.Equals, "Twilio Sandbox")
	c.Assert(got.FromNumber, check.Equals, "+15005550006")
	c.Assert(got.Provider, check.Equals, "twilio")

	byName, err := GetSMSProfileByName("Twilio Sandbox", 1)
	c.Assert(err, check.IsNil)
	c.Assert(byName.Id, check.Equals, p.Id)
}

func (s *ModelsSuite) TestSMSProfilePut(c *check.C) {
	p := SMSProfile{
		UserID:     1,
		Name:       "p1",
		Provider:   "twilio",
		AccountSID: "AC1",
		AuthToken:  "tok",
		FromNumber: "+15005550006",
	}
	c.Assert(PostSMSProfile(&p), check.IsNil)

	p.FromNumber = "+15005550007"
	c.Assert(PutSMSProfile(&p), check.IsNil)

	got, err := GetSMSProfile(p.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.FromNumber, check.Equals, "+15005550007")
}

func (s *ModelsSuite) TestSMSProfileDelete(c *check.C) {
	p := SMSProfile{
		UserID:     1,
		Name:       "p1",
		Provider:   "twilio",
		AccountSID: "AC1",
		AuthToken:  "tok",
		FromNumber: "+15005550006",
	}
	c.Assert(PostSMSProfile(&p), check.IsNil)
	c.Assert(DeleteSMSProfile(p.Id, 1), check.IsNil)

	_, err := GetSMSProfile(p.Id, 1)
	c.Assert(err, check.NotNil)
}
