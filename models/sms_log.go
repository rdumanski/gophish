package models

import (
	"time"
)

// SMSLog is the SMS-channel queue row, parallel to MailLog. Rows are
// created at campaign launch and consumed by the dispatcher (Phase 8b),
// which calls the configured sms.Sender implementation. The shape mirrors
// MailLog so the existing worker.processCampaigns loop can route both
// kinds of jobs through the same exponential-backoff machinery.
//
// Phase 8a defines the type, table, and the row-creation helper. The
// Backoff/Lock/Unlock/Error/Success methods and the Generate→sms.Job
// projection live in Phase 8b alongside the worker integration.
type SMSLog struct {
	Id          int64     `json:"-"`
	UserID      int64     `json:"-"`
	CampaignID  int64     `json:"campaign_id"`
	RID         string    `json:"id"`
	SendDate    time.Time `json:"send_date"`
	SendAttempt int       `json:"send_attempt"`
	Processing  bool      `json:"-"`
}

// TableName pins the GORM table name.
func (SMSLog) TableName() string { return "sms_logs" }

// GenerateSMSLog mirrors GenerateMailLog: one queue row per Result, with
// the initial send date matching the campaign schedule.
func GenerateSMSLog(c *Campaign, r *Result, sendDate time.Time) error {
	m := &SMSLog{
		UserID:     c.UserID,
		CampaignID: c.Id,
		RID:        r.RID,
		SendDate:   sendDate,
	}
	return db.Save(m).Error
}
