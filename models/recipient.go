package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Recipient is the canonical record for a single person, keyed by
// (UserID, Email). Unlike a Target (which is group-scoped and shared across
// users by content) or a Result (which is per-campaign), a Recipient persists
// across campaigns and is the entity later phases hang longitudinal state on:
// training enrollments, quiz scores, and risk over time (Phase 10b onward).
//
// Recipients are reconciled on write — PostCampaign upserts one per result —
// rather than via a historical data backfill. Rows created before Phase 10a
// link the next time a campaign including that email is created.
type Recipient struct {
	Id           int64     `json:"id" gorm:"primaryKey;column:id"`
	UserID       int64     `json:"-" gorm:"column:user_id"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Position     string    `json:"position"`
	Phone        string    `json:"phone"`
	CreatedDate  time.Time `json:"created_date"`
	ModifiedDate time.Time `json:"modified_date"`
}

// UpsertRecipient finds the Recipient for (userID, email) or creates it,
// refreshing the profile fields (name/position/phone) to the latest values
// seen. It returns the recipient's id. The work runs on the provided
// transaction so it composes with the caller's atomic write (e.g. campaign
// creation).
//
// An empty email means there is no stable key for a person, so the recipient
// link is skipped and (0, nil) is returned — the caller leaves recipient_id
// NULL. This is the phone-only / email-less target case (SMS, deferred).
//
// Invariant: this is the only sanctioned way to create a Recipient, and
// PostCampaign is currently the only path that persists a Result. The
// "every result links to a recipient" guarantee Phase 13 depends on holds
// only while both stay true — any new Result-creating path (e.g. in 10b/10c)
// must call UpsertRecipient too.
func UpsertRecipient(tx *gorm.DB, userID int64, br BaseRecipient) (int64, error) {
	if br.Email == "" {
		return 0, nil
	}
	now := time.Now().UTC()
	r := Recipient{}
	err := tx.Where("user_id = ? AND email = ?", userID, br.Email).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		r = Recipient{
			UserID:       userID,
			Email:        br.Email,
			FirstName:    br.FirstName,
			LastName:     br.LastName,
			Position:     br.Position,
			Phone:        br.Phone,
			CreatedDate:  now,
			ModifiedDate: now,
		}
		if err := tx.Create(&r).Error; err != nil {
			return 0, err
		}
		return r.Id, nil
	}
	if err != nil {
		return 0, err
	}
	// Refresh the canonical profile to the latest values (last-writer-wins).
	r.FirstName = br.FirstName
	r.LastName = br.LastName
	r.Position = br.Position
	if br.Phone != "" {
		r.Phone = br.Phone
	}
	r.ModifiedDate = now
	if err := tx.Save(&r).Error; err != nil {
		return 0, err
	}
	return r.Id, nil
}

// GetRecipientByEmail returns the Recipient owned by userID with the given
// email, or gorm.ErrRecordNotFound if none exists.
func GetRecipientByEmail(email string, userID int64) (Recipient, error) {
	r := Recipient{}
	err := db.Where("user_id = ? AND email = ?", userID, email).First(&r).Error
	return r, err
}

// GetRecipientByID returns the Recipient with the given id. Not user-scoped:
// the learner portal resolves the recipient from an enrollment whose token
// already authorized the request, so there is no operator session to scope
// against.
func GetRecipientByID(id int64) (Recipient, error) {
	r := Recipient{}
	err := db.Where("id = ?", id).First(&r).Error
	return r, err
}
