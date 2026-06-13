package models

import (
	"bytes"
	"net/mail"
	"time"

	"github.com/rdumanski/gophish/internal/gomail"
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestLearnerLink(ch *check.C) {
	ch.Assert(learnerLink("https://x.com", "tok"), check.Equals, "https://x.com/learn/tok")
	ch.Assert(learnerLink("https://x.com/", "tok"), check.Equals, "https://x.com/learn/tok")
}

func (s *ModelsSuite) TestBuildInvitationMessage(ch *check.C) {
	from, _ := mail.ParseAddress("Security Team <sec@corp.com>")
	msg := gomail.NewMessage()
	buildInvitationMessage(msg, *from, invitationData{
		ToEmail:    "lee@corp.com",
		ToName:     "Lee Example",
		FirstName:  "Lee",
		ModuleName: "Spotting Phishing",
		DueDate:    time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Link:       "https://learn.corp.com/learn/abc123",
	})

	ch.Assert(msg.GetHeader("Subject")[0], check.Equals, "Security awareness training: Spotting Phishing")
	ch.Assert(msg.GetHeader("To")[0], check.Equals, `"Lee Example" <lee@corp.com>`)

	var buf bytes.Buffer
	_, err := msg.WriteTo(&buf)
	ch.Assert(err, check.Equals, nil)
	body := buf.String()
	for _, want := range []string{"Hello Lee", "Spotting Phishing", "https://learn.corp.com/learn/abc123", "July 1, 2026"} {
		ch.Assert(bytes.Contains([]byte(body), []byte(want)), check.Equals, true)
	}
}

func (s *ModelsSuite) TestSendTrainingInvitationsValidates(ch *check.C) {
	module := s.makeModule(ch)
	s.makeGroupWithTargets(ch)
	tc := TrainingCampaign{Name: "C", ModuleID: module.Id, Groups: []Group{{Name: "TC Group"}}}
	ch.Assert(PostTrainingCampaign(&tc, 1), check.Equals, nil)

	// Missing base URL.
	_, err := SendTrainingInvitations(1, tc.Id, "Any", "")
	ch.Assert(err, check.Equals, ErrInvitationNoBaseURL)

	// Unknown SMTP profile.
	_, err = SendTrainingInvitations(1, tc.Id, "Nope", "https://x.com")
	ch.Assert(err, check.Equals, ErrSMTPNotFound)
}
