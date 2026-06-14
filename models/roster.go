package models

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/rdumanski/gophish/logger"
	"gorm.io/gorm"
)

// defaultMaxRemovalPercent is the share of a group's members a single sync may
// prune before it's refused as a likely truncated/partial export.
const defaultMaxRemovalPercent = 30

// RosterSource configures an email-to-mailbox roster sync: it watches an IMAP
// mailbox for a CSV attachment and upserts the named target group from it.
//
// Trust model: the mailbox is expected to be DEDICATED and access-controlled —
// its address is effectively the shared secret. SubjectToken is an additional
// shared secret that must appear in the email subject and IS the security
// control. FromFilter is a convenience filter only: From headers are trivially
// spoofable without DKIM/DMARC verification, so it must not be relied on as
// authentication.
type RosterSource struct {
	Id               int64  `json:"id" gorm:"primaryKey;column:id"`
	UserID           int64  `json:"-" gorm:"column:user_id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Host             string `json:"host"`
	Port             uint16 `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	TLS              bool   `json:"tls"`
	IgnoreCertErrors bool   `json:"ignore_cert_errors" gorm:"column:ignore_cert_errors"`
	Folder           string `json:"folder"`
	// FromFilter is a convenience sender filter (NOT authentication).
	FromFilter string `json:"from_filter" gorm:"column:from_filter"`
	// SubjectToken, if set, must be a substring of the email subject — the
	// shared-secret auth control for accepting a roster.
	SubjectToken string `json:"subject_token" gorm:"column:subject_token"`
	// TargetGroup is the name of the group whose membership this source syncs.
	TargetGroup string `json:"target_group" gorm:"column:target_group"`
	// MaxRemovalPercent caps how much of the group one sync may prune (guards
	// against a truncated CSV emptying the roster). <=0 means the default.
	MaxRemovalPercent int       `json:"max_removal_percent" gorm:"column:max_removal_percent"`
	Freq              uint32    `json:"freq"`
	LastSync          time.Time `json:"last_sync"`
	LastStatus        string    `json:"last_status" gorm:"column:last_status"`
	ModifiedDate      time.Time `json:"modified_date"`
}

// Roster validation errors.
var (
	ErrRosterNameNotSpecified  = errors.New("roster source name not specified")
	ErrRosterHostNotSpecified  = errors.New("imap host not specified")
	ErrRosterUserNotSpecified  = errors.New("imap username not specified")
	ErrRosterGroupNotSpecified = errors.New("target group not specified")
)

// RosterAttachment is one email attachment (testable input, no email lib dep).
type RosterAttachment struct {
	Filename string
	Content  []byte
}

// RosterMessage is the engine's view of a fetched email — populated from IMAP
// by the sync layer, or constructed directly in tests.
type RosterMessage struct {
	SeqNum      uint32
	Date        time.Time
	From        string
	Subject     string
	Attachments []RosterAttachment
}

// RosterSyncResult reports the outcome of a ProcessRoster run.
type RosterSyncResult struct {
	Applied bool   `json:"applied"`
	Total   int    `json:"total"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Message string `json:"message"`
	// ConsumedSeqNums are the messages the engine acted on; the caller marks
	// them read AFTER a successful run (at-least-once: a crash before marking
	// just reprocesses idempotently).
	ConsumedSeqNums []uint32 `json:"-"`
}

func (rs *RosterSource) effectiveMaxRemoval() int {
	if rs.MaxRemovalPercent <= 0 {
		return defaultMaxRemovalPercent
	}
	if rs.MaxRemovalPercent > 100 {
		return 100
	}
	return rs.MaxRemovalPercent
}

// Validate checks required fields and applies defaults.
func (rs *RosterSource) Validate() error {
	switch {
	case rs.Name == "":
		return ErrRosterNameNotSpecified
	case rs.Host == "":
		return ErrRosterHostNotSpecified
	case rs.Username == "":
		return ErrRosterUserNotSpecified
	case rs.TargetGroup == "":
		return ErrRosterGroupNotSpecified
	}
	if rs.Folder == "" {
		rs.Folder = "INBOX"
	}
	if rs.MaxRemovalPercent <= 0 {
		rs.MaxRemovalPercent = defaultMaxRemovalPercent
	}
	if rs.Freq == 0 {
		rs.Freq = 3600
	}
	return nil
}

func isCSVName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".csv") || strings.HasSuffix(n, ".txt")
}

func messageHasCSV(m RosterMessage) bool {
	for _, a := range m.Attachments {
		if isCSVName(a.Filename) {
			return true
		}
	}
	return false
}

// ProcessRoster applies a roster sync from a batch of fetched messages. It is
// pure of IMAP (DB-only) so it's fully unit-testable. It accepts only messages
// matching the subject token + sender filter that carry a CSV, takes the
// NEWEST such message (a roster is a full-state snapshot, so latest wins),
// refuses an over-threshold pruning, then upserts the target group.
func ProcessRoster(rs *RosterSource, msgs []RosterMessage) RosterSyncResult {
	accepted := make([]RosterMessage, 0, len(msgs))
	for _, m := range msgs {
		if rs.SubjectToken != "" && !strings.Contains(m.Subject, rs.SubjectToken) {
			continue
		}
		if rs.FromFilter != "" && !strings.Contains(strings.ToLower(m.From), strings.ToLower(rs.FromFilter)) {
			continue
		}
		if !messageHasCSV(m) {
			continue
		}
		accepted = append(accepted, m)
	}
	if len(accepted) == 0 {
		return RosterSyncResult{Applied: false, Message: "no matching roster email found"}
	}
	consumed := make([]uint32, 0, len(accepted))
	newest := accepted[0]
	for _, m := range accepted {
		consumed = append(consumed, m.SeqNum)
		if m.Date.After(newest.Date) {
			newest = m
		}
	}

	// Parse the newest message's CSV attachment(s) into deduped, emailed targets.
	seen := make(map[string]bool)
	targets := []Target{}
	for _, a := range newest.Attachments {
		if !isCSVName(a.Filename) {
			continue
		}
		rows, err := ParseCSVReader(bytes.NewReader(a.Content))
		if err != nil {
			continue
		}
		for _, t := range rows {
			key := strings.ToLower(strings.TrimSpace(t.Email))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return RosterSyncResult{Applied: false, Message: "roster CSV had no valid recipient rows", ConsumedSeqNums: consumed}
	}

	existing, err := GetGroupByName(rs.TargetGroup, rs.UserID)
	groupExists := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return RosterSyncResult{Applied: false, Message: "lookup failed: " + err.Error(), ConsumedSeqNums: consumed}
	}

	added, removed := len(targets), 0
	if groupExists {
		newSet := emailSet(targets)
		existingSet := emailSet(existing.Targets)
		added = 0
		for k := range newSet {
			if !existingSet[k] {
				added++
			}
		}
		for k := range existingSet {
			if !newSet[k] {
				removed++
			}
		}
		// Destructive-delta guard.
		if cur := len(existing.Targets); cur > 0 {
			if pct := removed * 100 / cur; pct > rs.effectiveMaxRemoval() {
				return RosterSyncResult{
					Applied:         false,
					ConsumedSeqNums: consumed,
					Message: fmt.Sprintf("refused: roster would remove %d of %d members (%d%% > %d%% limit) — likely a truncated export",
						removed, cur, pct, rs.effectiveMaxRemoval()),
				}
			}
		}
	}

	g := Group{Name: rs.TargetGroup, UserID: rs.UserID, Targets: targets}
	var applyErr error
	if groupExists {
		g.Id = existing.Id
		applyErr = PutGroup(&g)
	} else {
		applyErr = PostGroup(&g)
	}
	if applyErr != nil {
		return RosterSyncResult{Applied: false, ConsumedSeqNums: consumed, Message: "apply failed: " + applyErr.Error()}
	}
	// Keep the canonical recipient set current with the roster (so org fields
	// land on Recipients), then rebuild the per-org-unit auto-groups. Failures
	// here don't fail the sync — the membership update already succeeded.
	for _, t := range targets {
		if _, err := UpsertRecipient(db, rs.UserID, t.BaseRecipient); err != nil {
			log.Error(err)
		}
	}
	if _, err := RegenerateOrgGroups(rs.UserID); err != nil {
		log.Error(err)
	}
	return RosterSyncResult{
		Applied:         true,
		Total:           len(targets),
		Added:           added,
		Removed:         removed,
		ConsumedSeqNums: consumed,
		Message:         fmt.Sprintf("synced %d members (+%d, -%d)", len(targets), added, removed),
	}
}

func emailSet(ts []Target) map[string]bool {
	m := make(map[string]bool, len(ts))
	for _, t := range ts {
		if e := strings.ToLower(strings.TrimSpace(t.Email)); e != "" {
			m[e] = true
		}
	}
	return m
}

// RecordSyncResult persists the outcome timestamp + human-readable status.
func (rs *RosterSource) RecordSyncResult(status string) error {
	rs.LastSync = time.Now().UTC()
	rs.LastStatus = status
	return db.Model(&RosterSource{}).Where("id=?", rs.Id).
		Updates(map[string]interface{}{"last_sync": rs.LastSync, "last_status": rs.LastStatus}).Error
}

// GetRosterSources returns the operator's roster sources.
func GetRosterSources(uid int64) ([]RosterSource, error) {
	rss := []RosterSource{}
	err := db.Where("user_id=?", uid).Find(&rss).Error
	if err != nil {
		log.Error(err)
	}
	return rss, err
}

// GetEnabledRosterSources returns every enabled source across users (scheduler).
func GetEnabledRosterSources() ([]RosterSource, error) {
	rss := []RosterSource{}
	err := db.Where("enabled=?", true).Find(&rss).Error
	return rss, err
}

// GetRosterSource returns a single source scoped to its owner.
func GetRosterSource(id int64, uid int64) (RosterSource, error) {
	rs := RosterSource{}
	err := db.Where("user_id=? and id=?", uid, id).First(&rs).Error
	if err != nil {
		log.Error(err)
	}
	return rs, err
}

// PostRosterSource creates a roster source after validation.
func PostRosterSource(rs *RosterSource, uid int64) error {
	if err := rs.Validate(); err != nil {
		return err
	}
	rs.UserID = uid
	rs.ModifiedDate = time.Now().UTC()
	if err := db.Save(rs).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PutRosterSource updates a roster source after validation.
func PutRosterSource(rs *RosterSource, uid int64) error {
	if err := rs.Validate(); err != nil {
		return err
	}
	rs.UserID = uid
	rs.ModifiedDate = time.Now().UTC()
	if err := db.Where("id=?", rs.Id).Save(rs).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteRosterSource deletes a roster source by id, scoped to its owner.
func DeleteRosterSource(id int64, uid int64) error {
	if err := db.Where("user_id=?", uid).Delete(&RosterSource{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}
