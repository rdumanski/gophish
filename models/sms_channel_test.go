package models

import (
	"errors"
	"testing"

	"gopkg.in/check.v1"
)

// --- Group / Target: phone-aware acceptance ---------------------------------

func (s *ModelsSuite) TestPostGroupAcceptsPhoneOnlyTarget(c *check.C) {
	g := Group{Name: "SMS-only Group", UserID: 1}
	g.Targets = []Target{
		{BaseRecipient: BaseRecipient{Phone: "+15005550006"}},
	}
	c.Assert(PostGroup(&g), check.Equals, nil)
	got, err := GetGroup(g.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(len(got.Targets), check.Equals, 1)
	c.Assert(got.Targets[0].Phone, check.Equals, "+15005550006")
	c.Assert(got.Targets[0].Email, check.Equals, "")
}

func (s *ModelsSuite) TestPostGroupAcceptsEmailAndPhoneTarget(c *check.C) {
	g := Group{Name: "Mixed Group", UserID: 1}
	g.Targets = []Target{
		{BaseRecipient: BaseRecipient{Email: "alice@example.com", Phone: "+15005550006"}},
	}
	c.Assert(PostGroup(&g), check.Equals, nil)
	got, err := GetGroup(g.Id, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Targets[0].Phone, check.Equals, "+15005550006")
	c.Assert(got.Targets[0].Email, check.Equals, "alice@example.com")
}

func (s *ModelsSuite) TestPostGroupRejectsContactlessTarget(c *check.C) {
	g := Group{Name: "Empty Target Group", UserID: 1}
	g.Targets = []Target{
		{BaseRecipient: BaseRecipient{FirstName: "Nameless"}},
	}
	err := PostGroup(&g)
	c.Assert(errors.Is(err, ErrNoContactSpecified), check.Equals, true)
}

func (s *ModelsSuite) TestPostGroupRejectsBadPhone(c *check.C) {
	g := Group{Name: "Bad Phone Group", UserID: 1}
	g.Targets = []Target{
		{BaseRecipient: BaseRecipient{Phone: "555-555-1234"}},
	}
	err := PostGroup(&g)
	c.Assert(errors.Is(err, ErrInvalidPhone), check.Equals, true)
}

// --- Template: SMS channel rules -------------------------------------------

func TestTemplateValidateSMSRequiresText(t_ *testing.T) {
	t := Template{Name: "sms-no-body", Channel: "sms"}
	if err := t.Validate(); !errors.Is(err, ErrSMSTemplateMissingText) {
		t_.Errorf("expected ErrSMSTemplateMissingText, got %v", err)
	}
}

func TestTemplateValidateSMSRejectsEmailFields(t_ *testing.T) {
	cases := map[string]Template{
		"with subject":         {Name: "x", Channel: "sms", Text: "hi", Subject: "hi"},
		"with html":            {Name: "x", Channel: "sms", Text: "hi", HTML: "<p>hi</p>"},
		"with envelope sender": {Name: "x", Channel: "sms", Text: "hi", EnvelopeSender: "bot@example.com"},
		"with attachment": {
			Name: "x", Channel: "sms", Text: "hi",
			Attachments: []Attachment{{Name: "a.txt", Content: "Zm9v", Type: "text/plain"}},
		},
	}
	for label, tpl := range cases {
		err := tpl.Validate()
		if !errors.Is(err, ErrSMSTemplateHasEmailFields) {
			t_.Errorf("%s: expected ErrSMSTemplateHasEmailFields, got %v", label, err)
		}
	}
}

func TestTemplateValidateSMSAcceptsTextOnly(t_ *testing.T) {
	t := Template{Name: "sms-ok", Channel: "sms", Text: "hi {{.FirstName}} — {{.URL}}"}
	if err := t.Validate(); err != nil {
		t_.Errorf("expected nil, got %v", err)
	}
}

func TestTemplateValidateRejectsUnknownChannel(t_ *testing.T) {
	t := Template{Name: "x", Channel: "telegram", Text: "hi"}
	if err := t.Validate(); !errors.Is(err, ErrUnknownTemplateChannel) {
		t_.Errorf("expected ErrUnknownTemplateChannel, got %v", err)
	}
}

func TestTemplateValidateEmailDefaultUnchanged(t_ *testing.T) {
	// Empty Channel is treated as "email" — existing behavior must hold.
	t := Template{Name: "email-default", Text: "hi"}
	if err := t.Validate(); err != nil {
		t_.Errorf("empty Channel should validate as email, got %v", err)
	}
}

// --- Campaign: channel + sending profile + template-channel match ----------

func TestCampaignValidateSMSRequiresSMSProfile(t_ *testing.T) {
	c := Campaign{
		Name:     "c",
		Channel:  "sms",
		Groups:   []Group{{Name: "g"}},
		Template: Template{Name: "t", Channel: "sms", Text: "hi"},
		Page:     Page{Name: "p"},
	}
	if err := c.Validate(); !errors.Is(err, ErrSMSProfileNotSpecified) {
		t_.Errorf("expected ErrSMSProfileNotSpecified, got %v", err)
	}
}

// Channel-match tests previously lived as Validate() unit tests, but
// Validate runs against the user-supplied Campaign before the Template
// is loaded from the DB by name — at that point Template.Channel is
// always empty. The check now lives inside PostCampaign and is
// exercised end-to-end by TestPostCampaignChannelMismatch below, which
// goes through the real load path.

func (s *ModelsSuite) TestPostCampaignChannelMismatch(c *check.C) {
	// Build the standard email-channel deps (group + email template +
	// page + SMTP), then ask for an "sms" campaign against the email
	// template. PostCampaign loads the template by name, sees its
	// Channel is "email", and rejects with ErrChannelMismatch.
	camp := s.createCampaignDependencies(c)
	smsProfile := SMSProfile{
		UserID:     1,
		Name:       "T",
		Provider:   "twilio",
		AccountSID: "AC1",
		AuthToken:  "tok",
		FromNumber: "+15005550006",
	}
	c.Assert(PostSMSProfile(&smsProfile), check.IsNil)
	camp.Channel = "sms"
	camp.SMSProfile = smsProfile
	camp.SMTP = SMTP{} // unused for SMS but Validate would reject it being unset before the load
	err := PostCampaign(&camp, camp.UserID)
	c.Assert(errors.Is(err, ErrChannelMismatch), check.Equals, true)
}

func TestCampaignValidateEmailDefaultUnchanged(t_ *testing.T) {
	// Empty Channel must continue to work as "email" — backward-compat.
	c := Campaign{
		Name:     "c",
		Groups:   []Group{{Name: "g"}},
		Template: Template{Name: "t"}, // empty Channel = email
		Page:     Page{Name: "p"},
		SMTP:     SMTP{Name: "s"},
	}
	if err := c.Validate(); err != nil {
		t_.Errorf("empty Channel should validate, got %v", err)
	}
}

func TestCampaignValidateUnknownChannel(t_ *testing.T) {
	c := Campaign{
		Name:     "c",
		Channel:  "telegram",
		Groups:   []Group{{Name: "g"}},
		Template: Template{Name: "t"},
		Page:     Page{Name: "p"},
	}
	if err := c.Validate(); !errors.Is(err, ErrUnknownCampaignChannel) {
		t_.Errorf("expected ErrUnknownCampaignChannel, got %v", err)
	}
}
