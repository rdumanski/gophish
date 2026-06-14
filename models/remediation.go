package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	log "github.com/rdumanski/gophish/logger"
)

// Remediation trigger thresholds.
const (
	RemediationOnClickOrSubmit = "click_or_submit"
	RemediationOnSubmit        = "submit"
)

// RemediationSettings is the per-operator config for remediation auto-
// enrollment (Phase 20): when a recipient fails a simulation, assign a training
// module and email them the learner-portal link. One row per user.
type RemediationSettings struct {
	Id            int64     `json:"-" gorm:"primaryKey;column:id"`
	UserID        int64     `json:"-" gorm:"column:user_id"`
	Enabled       bool      `json:"enabled"`
	ModuleID      int64     `json:"module_id" gorm:"column:module_id"`
	TriggerOn     string    `json:"trigger_on" gorm:"column:trigger_on"`
	SMTPName      string    `json:"smtp_name" gorm:"column:smtp_name"`
	PortalBaseURL string    `json:"portal_base_url" gorm:"column:portal_base_url"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName pins the table (the pluralizer would otherwise produce a different
// name).
func (RemediationSettings) TableName() string { return "remediation_settings" }

// GetRemediationSettings returns the operator's settings, or a disabled
// zero-value (with sensible defaults) when none exist yet.
func GetRemediationSettings(uid int64) RemediationSettings {
	s := RemediationSettings{}
	if err := db.Where("user_id = ?", uid).First(&s).Error; err != nil {
		return RemediationSettings{UserID: uid, TriggerOn: RemediationOnClickOrSubmit}
	}
	return s
}

// PutRemediationSettings upserts the operator's settings (one row per user).
func PutRemediationSettings(s *RemediationSettings) error {
	if s.TriggerOn != RemediationOnSubmit {
		s.TriggerOn = RemediationOnClickOrSubmit
	}
	s.UpdatedAt = time.Now().UTC()
	existing := RemediationSettings{}
	if err := db.Where("user_id = ?", s.UserID).First(&existing).Error; err == nil {
		s.Id = existing.Id
	}
	return db.Save(s).Error
}

// triggerMet reports whether the result's status meets the configured threshold.
func (s RemediationSettings) triggerMet(status string) bool {
	if s.TriggerOn == RemediationOnSubmit {
		return status == EventDataSubmit
	}
	return status == EventClicked || status == EventDataSubmit
}

// TriggerRemediation auto-enrolls a recipient who failed a simulation into the
// configured remediation module and (best-effort) emails them the portal link.
// It is synchronous and idempotent; callers in the phishing request path should
// invoke it in a goroutine so SMTP never blocks the response. A no-op when
// remediation is disabled, the threshold isn't met, there's no recipient, or
// the recipient is already enrolled in the module.
func TriggerRemediation(rs Result) error {
	s := GetRemediationSettings(rs.UserID)
	if !s.Enabled || s.ModuleID == 0 {
		return nil
	}
	if !s.triggerMet(rs.Status) {
		return nil
	}
	if rs.RecipientID == 0 || strings.TrimSpace(rs.Email) == "" {
		return nil
	}
	// Idempotent: one remediation enrollment per recipient per module.
	existing := Enrollment{}
	err := db.Where("user_id = ? AND recipient_id = ? AND module_id = ?", rs.UserID, rs.RecipientID, s.ModuleID).
		First(&existing).Error
	if err == nil {
		return nil // already enrolled
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	module, err := GetTrainingModule(s.ModuleID, rs.UserID)
	if err != nil {
		return ErrEnrollmentModuleNotFound
	}
	enrollment, err := CreateEnrollmentByEmail(rs.UserID, rs.BaseRecipient, s.ModuleID)
	if err != nil {
		return err
	}
	log.Infof("remediation: enrolled %s in module %q after %s", rs.Email, module.Name, rs.Status)

	if s.SMTPName != "" && strings.TrimSpace(s.PortalBaseURL) != "" {
		if err := sendEnrollmentInvitation(rs.UserID, enrollment, module, s.SMTPName, s.PortalBaseURL); err != nil {
			log.Errorf("remediation: invite to %s failed: %s", rs.Email, err)
		}
	}
	return nil
}
