package models

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	log "github.com/rdumanski/gophish/logger"
	"gorm.io/gorm"
)

// Enrollment status values. Lifecycle is strictly forward:
// assigned -> started -> completed.
const (
	EnrollmentAssigned  = "assigned"
	EnrollmentStarted   = "started"
	EnrollmentCompleted = "completed"
)

// Enrollment links a Recipient (the person, Phase 10a) to a TrainingModule
// (the content, Phase 10b). It is the learner-side analogue of a Result: one
// row per (person, assigned module), carrying a long random Token that is the
// sole credential for the public learner portal at /learn/{token}.
type Enrollment struct {
	Id            int64     `json:"id" gorm:"primaryKey;column:id"`
	UserID        int64     `json:"-" gorm:"column:user_id"`
	RecipientID   int64     `json:"recipient_id" gorm:"column:recipient_id"`
	ModuleID      int64     `json:"module_id" gorm:"column:module_id"`
	Token         string    `json:"token" gorm:"column:token"`
	Status        string    `json:"status" gorm:"column:status"`
	AssignedDate  time.Time `json:"assigned_date"`
	StartedDate   time.Time `json:"started_date"`
	CompletedDate time.Time `json:"completed_date"`
	// TrainingCampaignID links the enrollment to the campaign that created it
	// (Phase 11a). Zero/NULL for standalone enrollments made via the 10c API.
	TrainingCampaignID int64 `json:"training_campaign_id" gorm:"column:training_campaign_id"`
}

// ErrEnrollmentRecipientRequired is returned when no recipient could be
// resolved for an enrollment (e.g. an empty email).
var ErrEnrollmentRecipientRequired = errors.New("enrollment requires a recipient email")

// ErrEnrollmentModuleNotFound is returned when the module to enroll in does
// not exist for the operator.
var ErrEnrollmentModuleNotFound = errors.New("training module not found")

// generateEnrollmentToken returns a 256-bit cryptographically random token,
// hex-encoded. This is the learner portal's only credential, so it must be
// unguessable — deliberately NOT the short enumerable scheme used for the
// phishing rid.
func generateEnrollmentToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateUniqueEnrollmentToken generates tokens until one is unused. A
// collision is astronomically unlikely at 256 bits; the loop just makes the
// unique-index guarantee explicit.
func generateUniqueEnrollmentToken(tx *gorm.DB) (string, error) {
	for {
		tok, err := generateEnrollmentToken()
		if err != nil {
			return "", err
		}
		err = tx.Where("token=?", tok).First(&Enrollment{}).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tok, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// createEnrollment upserts the recipient (Phase 10a) and inserts an enrollment
// on the provided transaction. campaignID is 0 for standalone enrollments.
// Callers validate module ownership and own the transaction so a whole
// campaign's enrollments commit atomically.
func createEnrollment(tx *gorm.DB, uid int64, br BaseRecipient, moduleID int64, campaignID int64) (Enrollment, error) {
	e := Enrollment{}
	recipientID, err := UpsertRecipient(tx, uid, br)
	if err != nil {
		return e, err
	}
	if recipientID == 0 {
		return e, ErrEnrollmentRecipientRequired
	}
	token, err := generateUniqueEnrollmentToken(tx)
	if err != nil {
		return e, err
	}
	e = Enrollment{
		UserID:             uid,
		RecipientID:        recipientID,
		ModuleID:           moduleID,
		TrainingCampaignID: campaignID,
		Token:              token,
		Status:             EnrollmentAssigned,
		AssignedDate:       time.Now().UTC(),
	}
	if err := tx.Save(&e).Error; err != nil {
		return e, err
	}
	return e, nil
}

// CreateEnrollmentByEmail upserts the recipient (Phase 10a) for the given
// email and creates a standalone enrollment in moduleID, in one transaction.
// It is the operator entry point used by the 10c enrollment API.
func CreateEnrollmentByEmail(uid int64, br BaseRecipient, moduleID int64) (Enrollment, error) {
	e := Enrollment{}
	if br.Email == "" {
		return e, ErrEnrollmentRecipientRequired
	}
	if _, err := GetTrainingModule(moduleID, uid); err != nil {
		return e, ErrEnrollmentModuleNotFound
	}
	tx := db.Begin()
	e, err := createEnrollment(tx, uid, br, moduleID, 0)
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return Enrollment{}, err
	}
	return e, tx.Commit().Error
}

// GetEnrollments returns the enrollments owned by the given operator.
func GetEnrollments(uid int64) ([]Enrollment, error) {
	es := []Enrollment{}
	err := db.Where("user_id=?", uid).Find(&es).Error
	if err != nil {
		log.Error(err)
	}
	return es, err
}

// GetEnrollment returns an enrollment by id, scoped to its operator.
func GetEnrollment(id int64, uid int64) (Enrollment, error) {
	e := Enrollment{}
	err := db.Where("user_id=? and id=?", uid, id).First(&e).Error
	if err != nil {
		log.Error(err)
	}
	return e, err
}

// GetEnrollmentByToken returns the enrollment for a learner-portal token.
// The token is the credential, so this is intentionally not user-scoped; a
// gorm.ErrRecordNotFound result is expected for bad tokens and not logged.
func GetEnrollmentByToken(token string) (Enrollment, error) {
	e := Enrollment{}
	err := db.Where("token=?", token).First(&e).Error
	return e, err
}

// DeleteEnrollment deletes an enrollment by id, scoped to its operator.
// Deleting (or regenerating) the row is how a portal token is revoked.
func DeleteEnrollment(id int64, uid int64) error {
	if err := db.Where("user_id=?", uid).Delete(&Enrollment{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// MarkStarted promotes assigned -> started and stamps StartedDate, but only
// from the assigned state. Revisiting the portal after starting or completing
// is a no-op: a completed enrollment must never be downgraded to started, or
// the completion state Phase 13 risk-scoring reads would be corrupted. This
// mirrors the downgrade guard in Result.HandleEmailOpened.
func (e *Enrollment) MarkStarted() error {
	if e.Status != EnrollmentAssigned {
		return nil
	}
	e.Status = EnrollmentStarted
	e.StartedDate = time.Now().UTC()
	return db.Save(e).Error
}

// MarkCompleted moves the enrollment to completed and stamps CompletedDate.
// Idempotent: completing an already-completed enrollment is a no-op. If the
// learner completes without a recorded start (e.g. a fast external module),
// StartedDate is backfilled so the timeline stays coherent.
func (e *Enrollment) MarkCompleted() error {
	if e.Status == EnrollmentCompleted {
		return nil
	}
	now := time.Now().UTC()
	if e.StartedDate.IsZero() {
		e.StartedDate = now
	}
	e.Status = EnrollmentCompleted
	e.CompletedDate = now
	return db.Save(e).Error
}
