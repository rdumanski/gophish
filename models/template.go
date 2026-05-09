package models

import (
	"errors"
	"net/mail"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// Template models hold the attributes for an email or SMS template to be
// sent to targets. The Channel field discriminates: "email" (default) uses
// Subject/HTML/Text/Attachments/EnvelopeSender as before; "sms" uses only
// Text and rejects the email-only fields at validation time.
type Template struct {
	Id             int64        `json:"id" gorm:"primaryKey;column:id"`
	UserID         int64        `json:"-" gorm:"column:user_id"`
	Name           string       `json:"name"`
	EnvelopeSender string       `json:"envelope_sender"`
	Subject        string       `json:"subject"`
	Text           string       `json:"text"`
	HTML           string       `json:"html" gorm:"column:html"`
	ModifiedDate   time.Time    `json:"modified_date"`
	Attachments    []Attachment `json:"attachments"`
	// GeneratedBy records the AI provider+model that drafted this template,
	// e.g. "anthropic:claude-sonnet-4-6". Empty for hand-written templates.
	GeneratedBy string `json:"generated_by,omitempty" gorm:"column:generated_by"`
	// Channel is "email" (default) or "sms". Phase 8a stores it; Phase 8b
	// wires it through to dispatcher selection and the editor UI.
	Channel string `json:"channel,omitempty" gorm:"column:channel;default:email"`
}

// ErrTemplateNameNotSpecified is thrown when a template name is not specified
var ErrTemplateNameNotSpecified = errors.New("template name not specified")

// ErrTemplateMissingParameter is thrown when a needed parameter is not provided
var ErrTemplateMissingParameter = errors.New("need to specify at least plaintext or HTML content")

// ErrSMSTemplateMissingText is thrown when an SMS template has no body.
var ErrSMSTemplateMissingText = errors.New("SMS template requires a text body")

// ErrSMSTemplateHasEmailFields is thrown when an SMS template carries fields
// that only make sense for email (Subject, HTML, Attachments, EnvelopeSender).
var ErrSMSTemplateHasEmailFields = errors.New("SMS template must not set subject, html, attachments, or envelope_sender")

// ErrUnknownTemplateChannel is thrown when Channel is set to anything other
// than "" / "email" / "sms".
var ErrUnknownTemplateChannel = errors.New("template channel must be \"email\" or \"sms\"")

// Validate checks the given template to make sure values are appropriate and complete
func (t *Template) Validate() error {
	if t.Name == "" {
		return ErrTemplateNameNotSpecified
	}
	switch t.Channel {
	case "", "email":
		// Email channel — keep the original validation contract.
		switch {
		case t.Text == "" && t.HTML == "":
			return ErrTemplateMissingParameter
		case t.EnvelopeSender != "":
			if _, err := mail.ParseAddress(t.EnvelopeSender); err != nil {
				return err
			}
		}
	case "sms":
		// SMS channel — only Text is meaningful. Email-only fields are
		// rejected so the editor UI doesn't accidentally store them.
		if t.Text == "" {
			return ErrSMSTemplateMissingText
		}
		if t.Subject != "" || t.HTML != "" || t.EnvelopeSender != "" || len(t.Attachments) > 0 {
			return ErrSMSTemplateHasEmailFields
		}
	default:
		return ErrUnknownTemplateChannel
	}
	if err := ValidateTemplate(t.HTML); err != nil {
		return err
	}
	if err := ValidateTemplate(t.Text); err != nil {
		return err
	}
	for _, a := range t.Attachments {
		if err := a.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// GetTemplates returns the templates owned by the given user.
func GetTemplates(uid int64) ([]Template, error) {
	ts := []Template{}
	err := db.Where("user_id=?", uid).Find(&ts).Error
	if err != nil {
		log.Error(err)
		return ts, err
	}
	for i := range ts {
		// Get Attachments
		err = db.Where("template_id=?", ts[i].Id).Find(&ts[i].Attachments).Error
		if err == nil && len(ts[i].Attachments) == 0 {
			ts[i].Attachments = make([]Attachment, 0)
		}
		if err != nil {
			log.Error(err)
			return ts, err
		}
	}
	return ts, err
}

// GetTemplate returns the template, if it exists, specified by the given id and user_id.
func GetTemplate(id int64, uid int64) (Template, error) {
	t := Template{}
	err := db.Where("user_id=? and id=?", uid, id).First(&t).Error
	if err != nil {
		log.Error(err)
		return t, err
	}

	// Get Attachments
	err = db.Where("template_id=?", t.Id).Find(&t.Attachments).Error
	if err != nil {
		log.Error(err)
		return t, err
	}
	if len(t.Attachments) == 0 {
		t.Attachments = make([]Attachment, 0)
	}
	return t, err
}

// GetTemplateByName returns the template, if it exists, specified by the given name and user_id.
func GetTemplateByName(n string, uid int64) (Template, error) {
	t := Template{}
	err := db.Where("user_id=? and name=?", uid, n).First(&t).Error
	if err != nil {
		log.Error(err)
		return t, err
	}

	// Get Attachments
	err = db.Where("template_id=?", t.Id).Find(&t.Attachments).Error
	if err != nil {
		log.Error(err)
		return t, err
	}
	if len(t.Attachments) == 0 {
		t.Attachments = make([]Attachment, 0)
	}
	return t, err
}

// PostTemplate creates a new template in the database.
func PostTemplate(t *Template) error {
	// Insert into the DB
	if err := t.Validate(); err != nil {
		return err
	}
	err := db.Save(t).Error
	if err != nil {
		log.Error(err)
		return err
	}

	// Save every attachment
	for i := range t.Attachments {
		t.Attachments[i].TemplateID = t.Id
		err := db.Save(&t.Attachments[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// PutTemplate edits an existing template in the database.
// Per the PUT Method RFC, it presumes all data for a template is provided.
func PutTemplate(t *Template) error {
	if err := t.Validate(); err != nil {
		return err
	}
	// Delete all attachments, and replace with new ones
	err := db.Where("template_id=?", t.Id).Delete(&Attachment{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	for i := range t.Attachments {
		t.Attachments[i].TemplateID = t.Id
		err := db.Save(&t.Attachments[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}

	// Save final template
	err = db.Where("id=?", t.Id).Save(t).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteTemplate deletes an existing template in the database.
// An error is returned if a template with the given user id and template id is not found.
func DeleteTemplate(id int64, uid int64) error {
	// Delete attachments
	err := db.Where("template_id=?", id).Delete(&Attachment{}).Error
	if err != nil {
		log.Error(err)
		return err
	}

	// Finally, delete the template itself
	err = db.Where("user_id=?", uid).Delete(&Template{Id: id}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
