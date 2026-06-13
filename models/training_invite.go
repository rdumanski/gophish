package models

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/rdumanski/gophish/internal/gomail"
	log "github.com/rdumanski/gophish/logger"
)

// ErrInvitationNoBaseURL is returned when no portal base URL is supplied.
var ErrInvitationNoBaseURL = errors.New("a base URL is required to build learner portal links")

// invitationData is the set of fields needed to render one invitation email.
type invitationData struct {
	ToEmail    string
	ToName     string
	FirstName  string
	ModuleName string
	DueDate    time.Time
	Link       string
}

// buildInvitationMessage fills a gomail.Message with a training-invitation
// email. It is a pure function (no DB / network) so the rendered content can be
// asserted in tests. The body is plain first-party copy — operator-authored
// invitation templates are a later enhancement.
func buildInvitationMessage(msg *gomail.Message, from mail.Address, d invitationData) {
	msg.SetAddressHeader("From", from.Address, from.Name)
	if d.ToName != "" {
		msg.SetAddressHeader("To", d.ToEmail, d.ToName)
	} else {
		msg.SetHeader("To", d.ToEmail)
	}
	msg.SetHeader("Subject", "Security awareness training: "+d.ModuleName)

	greeting := "Hello"
	if d.FirstName != "" {
		greeting = "Hello " + d.FirstName
	}
	due := ""
	if !d.DueDate.IsZero() {
		due = fmt.Sprintf("\n\nPlease complete it by %s.", d.DueDate.Format("January 2, 2006"))
	}
	text := fmt.Sprintf("%s,\n\nYou have been assigned the security-awareness module \"%s\".\n\nStart it here:\n%s%s\n\nThank you.",
		greeting, d.ModuleName, d.Link, due)
	msg.SetBody("text/plain", text)

	dueHTML := ""
	if !d.DueDate.IsZero() {
		dueHTML = fmt.Sprintf("<p>Please complete it by <strong>%s</strong>.</p>", d.DueDate.Format("January 2, 2006"))
	}
	html := fmt.Sprintf(`<p>%s,</p><p>You have been assigned the security-awareness module <strong>%s</strong>.</p>`+
		`<p><a href="%s">Start the training &rarr;</a></p>%s<p>Thank you.</p>`,
		greeting, d.ModuleName, d.Link, dueHTML)
	msg.AddAlternative("text/html", html)
}

// learnerLink builds the absolute learner-portal URL for a token given an
// operator-supplied base URL.
func learnerLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/learn/" + token
}

// SendTrainingInvitations emails every enrollee in a training campaign their
// learner-portal link, via the chosen SMTP profile. It reuses the low-level
// SMTP dialer + gomail directly (roadmap Decision B) rather than the phishing
// Campaign/MailLog path. Sends are synchronous and batched over one connection;
// each successfully sent invitation stamps the enrollment's invited_date.
// Returns the number of invitations sent.
func SendTrainingInvitations(uid int64, campaignID int64, smtpName string, baseURL string) (int, error) {
	if strings.TrimSpace(baseURL) == "" {
		return 0, ErrInvitationNoBaseURL
	}
	tc, err := GetTrainingCampaign(campaignID, uid)
	if err != nil {
		return 0, err
	}
	module, err := GetTrainingModule(tc.ModuleID, uid)
	if err != nil {
		return 0, ErrEnrollmentModuleNotFound
	}
	smtp, err := GetSMTPByName(smtpName, uid)
	if err != nil {
		return 0, ErrSMTPNotFound
	}
	from, err := mail.ParseAddress(smtp.FromAddress)
	if err != nil {
		return 0, ErrInvalidFromAddress
	}

	enrollments := []Enrollment{}
	if err := db.Where("training_campaign_id=? AND user_id=?", campaignID, uid).Find(&enrollments).Error; err != nil {
		return 0, err
	}

	dialer, err := smtp.GetDialer()
	if err != nil {
		return 0, err
	}
	sender, err := dialer.Dial()
	if err != nil {
		return 0, err
	}
	defer func() { _ = sender.Close() }()

	sent := 0
	for i := range enrollments {
		e := &enrollments[i]
		recipient, err := GetRecipientByID(e.RecipientID)
		if err != nil {
			log.Errorf("invitation: recipient %d not found: %s", e.RecipientID, err)
			continue
		}
		msg := gomail.NewMessage()
		buildInvitationMessage(msg, *from, invitationData{
			ToEmail:    recipient.Email,
			ToName:     strings.TrimSpace(recipient.FirstName + " " + recipient.LastName),
			FirstName:  recipient.FirstName,
			ModuleName: module.Name,
			DueDate:    tc.DueDate,
			Link:       learnerLink(baseURL, e.Token),
		})
		if err := gomail.SendCustomFrom(sender, from.Address, msg); err != nil {
			log.Errorf("invitation: send to %s failed: %s", recipient.Email, err)
			continue
		}
		e.InvitedDate = time.Now().UTC()
		if err := db.Save(e).Error; err != nil {
			log.Error(err)
		}
		sent++
	}
	return sent, nil
}
