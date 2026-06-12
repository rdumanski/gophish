package models

import (
	"errors"
	"net/url"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// Training module content types. They control how the learner portal
// (Phase 10c) renders the module: inline HTML body, an embedded hosted
// video, or a link out to an external course.
const (
	TrainingContentHTML     = "html"
	TrainingContentVideo    = "video"
	TrainingContentExternal = "external"
)

// TrainingModule is a reusable piece of awareness-training content authored
// by an operator and later assigned to recipients (enrollment, Phase 10c).
// It is the training-side analogue of an email Template.
type TrainingModule struct {
	Id           int64     `json:"id" gorm:"primaryKey;column:id"`
	UserID       int64     `json:"-" gorm:"column:user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	ContentType  string    `json:"content_type" gorm:"column:content_type"`
	HTML         string    `json:"html" gorm:"column:html"`
	URL          string    `json:"url" gorm:"column:url"`
	ModifiedDate time.Time `json:"modified_date"`
}

// ErrTrainingNameNotSpecified is returned when a module has no name.
var ErrTrainingNameNotSpecified = errors.New("training module name not specified")

// ErrTrainingUnknownContentType is returned for an unrecognised content_type.
var ErrTrainingUnknownContentType = errors.New(`training module content_type must be "html", "video", or "external"`)

// ErrTrainingMissingHTML is returned when an html module has no body.
var ErrTrainingMissingHTML = errors.New("html training module requires an html body")

// ErrTrainingMissingURL is returned when a video/external module has no URL.
var ErrTrainingMissingURL = errors.New("video/external training module requires a url")

// ErrTrainingInvalidURL is returned when a module URL isn't absolute http(s).
var ErrTrainingInvalidURL = errors.New("training module url must be an absolute http(s) URL")

// Validate checks that the module is well-formed for its content type.
func (m *TrainingModule) Validate() error {
	if m.Name == "" {
		return ErrTrainingNameNotSpecified
	}
	switch m.ContentType {
	case TrainingContentHTML:
		if m.HTML == "" {
			return ErrTrainingMissingHTML
		}
	case TrainingContentVideo, TrainingContentExternal:
		if m.URL == "" {
			return ErrTrainingMissingURL
		}
		u, err := url.ParseRequestURI(m.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return ErrTrainingInvalidURL
		}
	default:
		return ErrTrainingUnknownContentType
	}
	return nil
}

// GetTrainingModules returns the training modules owned by the given user.
func GetTrainingModules(uid int64) ([]TrainingModule, error) {
	ms := []TrainingModule{}
	err := db.Where("user_id=?", uid).Find(&ms).Error
	if err != nil {
		log.Error(err)
	}
	return ms, err
}

// GetTrainingModule returns the module specified by id and user_id.
func GetTrainingModule(id int64, uid int64) (TrainingModule, error) {
	m := TrainingModule{}
	err := db.Where("user_id=? and id=?", uid, id).First(&m).Error
	if err != nil {
		log.Error(err)
	}
	return m, err
}

// GetTrainingModuleByName returns the module with the given name for a user.
// A gorm.ErrRecordNotFound result is expected by callers (uniqueness check)
// and is therefore not logged here.
func GetTrainingModuleByName(n string, uid int64) (TrainingModule, error) {
	m := TrainingModule{}
	err := db.Where("user_id=? and name=?", uid, n).First(&m).Error
	return m, err
}

// PostTrainingModule creates a new training module after validation.
func PostTrainingModule(m *TrainingModule) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := db.Save(m).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PutTrainingModule updates an existing training module after validation.
func PutTrainingModule(m *TrainingModule) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := db.Where("id=?", m.Id).Save(m).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteTrainingModule deletes a module by id, scoped to its owner.
func DeleteTrainingModule(id int64, uid int64) error {
	if err := db.Where("user_id=?", uid).Delete(&TrainingModule{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}
