package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// SMSProfile is the SMS analog of SMTP — a per-user, named provider
// configuration that an SMS campaign references when dispatching messages.
//
// Provider is a discriminator the dispatcher (Phase 8b) reads to pick the
// right sms.Sender implementation. Today only "twilio" is recognised; new
// providers slot in without a schema change.
//
// FromNumber is the E.164 sender number registered with the provider.
// MessagingServiceSID is Twilio-specific (lets Twilio choose a number out
// of a pool); when set, FromNumber may be empty. Validation requires at
// least one to be present.
//
// AuthToken is stored in plaintext, mirroring the way SMTP.Password is
// stored today. A future hardening pass (out of scope for 8a) would
// encrypt at rest behind a KEK.
type SMSProfile struct {
	Id                  int64     `json:"id" gorm:"primaryKey;column:id"`
	UserID              int64     `json:"-" gorm:"column:user_id"`
	Name                string    `json:"name"`
	Provider            string    `json:"provider" gorm:"column:provider;default:twilio"`
	AccountSID          string    `json:"account_sid" gorm:"column:account_sid"`
	AuthToken           string    `json:"auth_token,omitempty" gorm:"column:auth_token"`
	FromNumber          string    `json:"from_number" gorm:"column:from_number"`
	MessagingServiceSID string    `json:"messaging_service_sid,omitempty" gorm:"column:messaging_service_sid"`
	ModifiedDate        time.Time `json:"modified_date"`
}

// TableName pins the GORM table name (the convention pluraliser would
// produce "sms_profiles" anyway, but pinning matches the project style).
func (SMSProfile) TableName() string { return "sms_profiles" }

// SupportedSMSProviders is the set of provider strings the dispatcher
// knows how to instantiate. Phase 8a keeps this list as data so adding a
// provider in 8b/8c is a single-file change. Exported for tests.
var SupportedSMSProviders = map[string]bool{
	"twilio": true,
}

// ErrSMSProfileNameNotSpecified is thrown when no name is provided.
var ErrSMSProfileNameNotSpecified = errors.New("no SMS profile name specified")

// ErrSMSProfileProviderUnsupported wraps the offending provider string.
var ErrSMSProfileProviderUnsupported = errors.New("unsupported SMS provider")

// ErrSMSProfileMissingCredentials means AccountSID and/or AuthToken were
// empty. Provider-specific sub-fields are validated separately.
var ErrSMSProfileMissingCredentials = errors.New("missing provider credentials (account_sid + auth_token)")

// ErrSMSProfileMissingFromNumber means neither a from_number nor a
// messaging_service_sid was supplied. The dispatcher needs at least one.
var ErrSMSProfileMissingFromNumber = errors.New("either from_number or messaging_service_sid is required")

// Validate ensures the profile is fully populated for the chosen provider.
// Empty Provider defaults to "twilio" so existing-shape JSON keeps working.
func (s *SMSProfile) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return ErrSMSProfileNameNotSpecified
	}
	if s.Provider == "" {
		s.Provider = "twilio"
	}
	if !SupportedSMSProviders[s.Provider] {
		return fmt.Errorf("%w: %q", ErrSMSProfileProviderUnsupported, s.Provider)
	}
	if s.AccountSID == "" || s.AuthToken == "" {
		return ErrSMSProfileMissingCredentials
	}
	if s.FromNumber == "" && s.MessagingServiceSID == "" {
		return ErrSMSProfileMissingFromNumber
	}
	if s.FromNumber != "" {
		if err := ValidatePhone(s.FromNumber); err != nil {
			return fmt.Errorf("from_number: %w", err)
		}
	}
	return nil
}

// GetSMSProfiles returns every SMS profile owned by uid.
func GetSMSProfiles(uid int64) ([]SMSProfile, error) {
	ps := []SMSProfile{}
	err := db.Where("user_id=?", uid).Find(&ps).Error
	if err != nil {
		log.Error(err)
	}
	return ps, err
}

// GetSMSProfile returns the SMS profile by (id, uid) or an error if missing.
func GetSMSProfile(id int64, uid int64) (SMSProfile, error) {
	p := SMSProfile{}
	err := db.Where("user_id=? and id=?", uid, id).First(&p).Error
	if err != nil {
		log.Error(err)
	}
	return p, err
}

// GetSMSProfileByName looks up by (name, uid) — used by Campaign launch
// when the JSON references a profile by name.
func GetSMSProfileByName(n string, uid int64) (SMSProfile, error) {
	p := SMSProfile{}
	err := db.Where("user_id=? and name=?", uid, n).First(&p).Error
	if err != nil {
		log.Error(err)
	}
	return p, err
}

// PostSMSProfile persists a new profile after validation.
func PostSMSProfile(s *SMSProfile) error {
	if err := s.Validate(); err != nil {
		return err
	}
	s.ModifiedDate = time.Now().UTC()
	err := db.Create(s).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// PutSMSProfile updates an existing profile after validation.
func PutSMSProfile(s *SMSProfile) error {
	if err := s.Validate(); err != nil {
		return err
	}
	s.ModifiedDate = time.Now().UTC()
	err := db.Where("id=?", s.Id).Save(s).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// DeleteSMSProfile removes the profile if it belongs to uid.
func DeleteSMSProfile(id int64, uid int64) error {
	err := db.Where("user_id=?", uid).Delete(&SMSProfile{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}
