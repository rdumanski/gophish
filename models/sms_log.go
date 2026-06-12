package models

import (
	"errors"
	"fmt"
	"math"
	"time"

	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/sms"
	"gorm.io/gorm"
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

	// cachedCampaign mirrors MailLog.cachedCampaign — set by
	// CacheCampaign so the worker doesn't re-query the DB for every
	// recipient in a batch.
	cachedCampaign *Campaign `gorm:"-"`
}

// TableName pins the GORM table name.
func (SMSLog) TableName() string { return "sms_logs" }

// Compile-time assertion that *SMSLog satisfies sms.Job. If a method
// signature drifts (e.g. Receipt shape changes in 8c), the build
// breaks here rather than at the worker call site.
var _ sms.Job = (*SMSLog)(nil)

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

// MaxSMSSendAttempts caps the exponential backoff at the same 8 attempts
// MailLog uses (~4.2h max delay). Same rationale: provider hiccups are
// usually short, recipient-permanent failures should be detected by
// sentinel errors and short-circuit before hitting the cap.
var MaxSMSSendAttempts = MaxSendAttempts

// ErrMaxSMSSendAttempts mirrors ErrMaxSendAttempts. Sentinel for the
// dispatcher to treat the failure as terminal.
var ErrMaxSMSSendAttempts = errors.New("max SMS send attempts exceeded")

// Backoff bumps SendAttempt, pushes the next SendDate out by an
// exponential delay, records an SMSError event, and unlocks the row.
// Returns ErrMaxSMSSendAttempts once the cap is hit so the dispatcher
// knows to call Error() instead.
func (m *SMSLog) Backoff(reason error) error {
	r, err := GetResult(m.RID)
	if err != nil {
		return err
	}
	if m.SendAttempt == MaxSMSSendAttempts {
		if err := r.HandleSMSError(ErrMaxSMSSendAttempts); err != nil {
			log.Errorf("error recording SMS send-error event for result %s: %s", m.RID, err)
		}
		return ErrMaxSMSSendAttempts
	}
	m.SendAttempt++
	backoffDuration := math.Pow(2, float64(m.SendAttempt))
	m.SendDate = m.SendDate.Add(time.Minute * time.Duration(backoffDuration))
	if err := db.Save(m).Error; err != nil {
		return err
	}
	if err := r.HandleSMSBackoff(reason, m.SendDate); err != nil {
		return err
	}
	return m.Unlock()
}

// Unlock clears the processing flag so the dispatcher can pick the row
// up on the next poll.
func (m *SMSLog) Unlock() error {
	m.Processing = false
	return db.Save(m).Error
}

// Lock sets the processing flag so concurrent dispatcher loops don't
// re-pick the row.
func (m *SMSLog) Lock() error {
	m.Processing = true
	return db.Save(m).Error
}

// Error finalises the queue row as a permanent failure. Records the
// terminal SMSError event on the underlying Result and deletes the
// queue row (mirrors MailLog.Error).
func (m *SMSLog) Error(e error) error {
	r, err := GetResult(m.RID)
	if err != nil {
		log.Warn(err)
		return err
	}
	if err := r.HandleSMSError(e); err != nil {
		log.Warn(err)
		return err
	}
	return db.Delete(m).Error
}

// Success finalises the queue row as accepted-by-provider. Records
// EventSMSSent on the Result with the provider receipt as audit
// breadcrumb, then deletes the queue row. Receipt's ProviderID is the
// Twilio MessageSID (or equivalent) used by Phase 8c's status-callback
// reconciliation.
//
// Signature matches sms.Job — the interface forces every dispatcher
// callback to thread through the provider's receipt rather than
// silently dropping it.
func (m *SMSLog) Success(receipt sms.Receipt) error {
	r, err := GetResult(m.RID)
	if err != nil {
		return err
	}
	d := EventDetails{Browser: map[string]string{
		"provider_id": receipt.ProviderID,
		"status":      receipt.Status,
	}}
	if err := r.HandleSMSSent(d); err != nil {
		return err
	}
	return db.Delete(m).Error
}

// Render builds the per-recipient sms.Message — the To from
// Result.Phone, the From from the campaign's SMSProfile, the Body
// from the rendered Template.Text. Implements sms.Job.
func (m *SMSLog) Render() (sms.Message, error) {
	r, err := GetResult(m.RID)
	if err != nil {
		return sms.Message{}, err
	}
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignSMSContext(m.CampaignID, m.UserID)
		if err != nil {
			return sms.Message{}, err
		}
		c = &campaign
	}
	if r.Phone == "" {
		// Defensive — PostCampaign won't queue an SMSLog for a
		// phone-less target, but a manually-edited DB row could
		// land here.
		return sms.Message{}, fmt.Errorf("sms_log: result %s has no phone", m.RID)
	}
	ptx, err := NewPhishingTemplateContext(c, r.BaseRecipient, r.RID)
	if err != nil {
		return sms.Message{}, err
	}
	body, err := ExecuteTemplate(c.Template.Text, ptx)
	if err != nil {
		return sms.Message{}, err
	}
	return sms.Message{
		To:   r.Phone,
		From: c.SMSProfile.FromNumber,
		Body: body,
	}, nil
}

// GetSender constructs the provider client this campaign uses.
// Implements sms.Job.
func (m *SMSLog) GetSender() (sms.Sender, error) {
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignSMSContext(m.CampaignID, m.UserID)
		if err != nil {
			return nil, err
		}
		c = &campaign
	}
	return c.SMSProfile.Sender()
}

// CacheCampaign — same shape as MailLog.CacheCampaign. Lets the
// worker prebuild the campaign once per batch.
func (m *SMSLog) CacheCampaign(c *Campaign) error {
	if c.Id != m.CampaignID {
		return fmt.Errorf("sms_log: incorrect campaign provided for caching. expected %d got %d", m.CampaignID, c.Id)
	}
	m.cachedCampaign = c
	return nil
}

// GetSMSLog returns the SMSLog tied to the given RID. Returns
// gorm.ErrRecordNotFound when the queue row has already been
// finalised (Success/Error both delete it).
func GetSMSLog(rid string) (*SMSLog, error) {
	m := &SMSLog{}
	err := db.Where("r_id = ?", rid).First(m).Error
	return m, err
}

// GetSMSLogsByCampaign returns every queued SMS row for a campaign.
// Mirrors GetMailLogsByCampaign — used by LaunchCampaign to drive an
// initial dispatch immediately after the campaign goes live, rather
// than waiting up to a minute for the next processCampaigns tick.
func GetSMSLogsByCampaign(cid int64) ([]*SMSLog, error) {
	ms := []*SMSLog{}
	err := db.Where("campaign_id = ?", cid).Find(&ms).Error
	return ms, err
}

// GetQueuedSMSLogs returns all SMSLog rows whose SendDate has elapsed
// and which are not already locked by another dispatcher iteration.
// Mirrors GetQueuedMailLogs.
func GetQueuedSMSLogs(t time.Time) ([]*SMSLog, error) {
	ms := []*SMSLog{}
	err := db.Where("send_date <= ? AND processing = ?", t, false).Find(&ms).Error
	if err != nil {
		log.Error(err)
	}
	return ms, err
}

// LockSMSLogs flips Processing on every row in the supplied batch.
// Mirrors LockMailLogs's signature (lock bool) so the dispatcher can
// reuse the same lock-then-process-then-unlock pattern on either
// queue type.
func LockSMSLogs(ms []*SMSLog, lock bool) error {
	tx := db.Begin()
	for i := range ms {
		ms[i].Processing = lock
		if err := tx.Save(ms[i]).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// UnlockAllSMSLogs clears every leftover Processing=true flag on
// startup. Mirrors UnlockAllMailLogs — gorm v2 requires the explicit
// AllowGlobalUpdate opt-in for the no-WHERE UPDATE.
func UnlockAllSMSLogs() error {
	return db.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Model(&SMSLog{}).Update("processing", false).Error
}
