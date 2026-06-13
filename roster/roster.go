// Package roster implements the email-to-mailbox roster sync: it fetches a
// mailbox over IMAP, hands the messages to the (IMAP-free, testable)
// models.ProcessRoster engine, and marks the consumed messages read only after
// a successful apply.
package roster

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/rdumanski/gophish/imap"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// Sync runs one roster sync for the given source: fetch unread → process →
// mark read → record status. It never marks messages read on an IMAP/parse
// failure, so a transient error just reprocesses idempotently next time.
func Sync(rs *models.RosterSource) models.RosterSyncResult {
	host := rs.Host
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:%d", host, rs.Port)
	}
	mbox := imap.Mailbox{
		Host:             host,
		TLS:              rs.TLS,
		IgnoreCertErrors: rs.IgnoreCertErrors,
		User:             rs.Username,
		Pwd:              rs.Password,
		Folder:           rs.Folder,
	}

	// Fetch WITHOUT marking read; we mark read after a successful apply.
	emails, err := mbox.GetUnread(false, false)
	if err != nil {
		res := models.RosterSyncResult{Applied: false, Message: "imap error: " + err.Error()}
		_ = rs.RecordSyncResult(res.Message)
		return res
	}

	msgs := make([]models.RosterMessage, 0, len(emails))
	for _, e := range emails {
		date, _ := mail.ParseDate(e.Headers.Get("Date"))
		atts := make([]models.RosterAttachment, 0, len(e.Attachments))
		for _, a := range e.Attachments {
			atts = append(atts, models.RosterAttachment{Filename: a.Filename, Content: a.Content})
		}
		msgs = append(msgs, models.RosterMessage{
			SeqNum:      e.SeqNum,
			Date:        date,
			From:        e.From,
			Subject:     e.Subject,
			Attachments: atts,
		})
	}

	res := models.ProcessRoster(rs, msgs)
	if len(res.ConsumedSeqNums) > 0 {
		if err := mbox.MarkAsRead(res.ConsumedSeqNums); err != nil {
			log.Error("roster: mark-as-read failed: ", err)
		}
	}
	_ = rs.RecordSyncResult(res.Message)
	return res
}
