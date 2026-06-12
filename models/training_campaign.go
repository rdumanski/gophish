package models

import (
	"errors"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// TrainingCampaign assigns one TrainingModule to one or more Groups with a due
// date. Creating it bulk-creates an Enrollment per unique recipient across the
// selected groups. Completion is aggregated from those enrollments.
type TrainingCampaign struct {
	Id          int64                 `json:"id" gorm:"primaryKey;column:id"`
	UserID      int64                 `json:"-" gorm:"column:user_id"`
	Name        string                `json:"name"`
	ModuleID    int64                 `json:"module_id" gorm:"column:module_id"`
	DueDate     time.Time             `json:"due_date"`
	CreatedDate time.Time             `json:"created_date"`
	Groups      []Group               `json:"groups,omitempty" gorm:"-"`
	Stats       TrainingCampaignStats `json:"stats" gorm:"-"`
}

// TrainingCampaignStats is the per-campaign completion breakdown.
type TrainingCampaignStats struct {
	Total     int64 `json:"total"`
	Assigned  int64 `json:"assigned"`
	Started   int64 `json:"started"`
	Completed int64 `json:"completed"`
}

// ErrTrainingCampaignNameNotSpecified is returned when a campaign has no name.
var ErrTrainingCampaignNameNotSpecified = errors.New("training campaign name not specified")

// ErrTrainingCampaignNoGroups is returned when no groups are specified.
var ErrTrainingCampaignNoGroups = errors.New("no groups specified")

// PostTrainingCampaign creates a training campaign and bulk-enrolls every
// unique recipient across the selected groups in the campaign's module.
func PostTrainingCampaign(tc *TrainingCampaign, uid int64) error {
	if tc.Name == "" {
		return ErrTrainingCampaignNameNotSpecified
	}
	if len(tc.Groups) == 0 {
		return ErrTrainingCampaignNoGroups
	}
	if _, err := GetTrainingModule(tc.ModuleID, uid); err != nil {
		return ErrEnrollmentModuleNotFound
	}

	// Resolve all groups + dedupe targets BEFORE opening the transaction.
	// Reads must not run on `db` while a tx holds the single sqlite
	// connection (SetMaxOpenConns(1)); PostCampaign follows the same shape.
	seen := make(map[string]bool)
	recipients := []BaseRecipient{}
	for _, g := range tc.Groups {
		group, err := GetGroupByName(g.Name, uid)
		if err != nil {
			return ErrGroupNotFound
		}
		for _, t := range group.Targets {
			if t.Email == "" || seen[t.Email] {
				continue
			}
			seen[t.Email] = true
			recipients = append(recipients, t.BaseRecipient)
		}
	}

	tx := db.Begin()
	tc.UserID = uid
	tc.CreatedDate = time.Now().UTC()
	if err := tx.Save(tc).Error; err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}
	for _, br := range recipients {
		if _, err := createEnrollment(tx, uid, br, tc.ModuleID, tc.Id); err != nil {
			tx.Rollback()
			log.Error(err)
			return err
		}
	}
	return tx.Commit().Error
}

// trainingCampaignStats aggregates enrollment statuses for a campaign.
func trainingCampaignStats(campaignID int64) TrainingCampaignStats {
	s := TrainingCampaignStats{}
	base := db.Model(&Enrollment{}).Where("training_campaign_id=?", campaignID)
	base.Count(&s.Total)
	db.Model(&Enrollment{}).Where("training_campaign_id=? AND status=?", campaignID, EnrollmentAssigned).Count(&s.Assigned)
	db.Model(&Enrollment{}).Where("training_campaign_id=? AND status=?", campaignID, EnrollmentStarted).Count(&s.Started)
	db.Model(&Enrollment{}).Where("training_campaign_id=? AND status=?", campaignID, EnrollmentCompleted).Count(&s.Completed)
	return s
}

// GetTrainingCampaigns returns the operator's campaigns with completion stats.
func GetTrainingCampaigns(uid int64) ([]TrainingCampaign, error) {
	tcs := []TrainingCampaign{}
	if err := db.Where("user_id=?", uid).Find(&tcs).Error; err != nil {
		log.Error(err)
		return tcs, err
	}
	for i := range tcs {
		tcs[i].Stats = trainingCampaignStats(tcs[i].Id)
	}
	return tcs, nil
}

// GetTrainingCampaign returns a single campaign with completion stats.
func GetTrainingCampaign(id int64, uid int64) (TrainingCampaign, error) {
	tc := TrainingCampaign{}
	if err := db.Where("user_id=? and id=?", uid, id).First(&tc).Error; err != nil {
		log.Error(err)
		return tc, err
	}
	tc.Stats = trainingCampaignStats(tc.Id)
	return tc, nil
}

// DeleteTrainingCampaign deletes a campaign and the enrollments it created.
func DeleteTrainingCampaign(id int64, uid int64) error {
	if _, err := GetTrainingCampaign(id, uid); err != nil {
		return err
	}
	if err := db.Where("training_campaign_id=?", id).Delete(&Enrollment{}).Error; err != nil {
		log.Error(err)
		return err
	}
	if err := db.Where("user_id=?", uid).Delete(&TrainingCampaign{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}
