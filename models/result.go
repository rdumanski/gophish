package models

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
	log "github.com/rdumanski/gophish/logger"
	"gorm.io/gorm"
)

type mmCity struct {
	GeoPoint mmGeoPoint `maxminddb:"location"`
}

type mmGeoPoint struct {
	Latitude  float64 `maxminddb:"latitude"`
	Longitude float64 `maxminddb:"longitude"`
}

// Result contains the fields for a result object,
// which is a representation of a target in a campaign.
type Result struct {
	Id           int64     `json:"-"`
	CampaignID   int64     `json:"-"`
	UserID       int64     `json:"-"`
	RID          string    `json:"id"`
	Status       string    `json:"status" gorm:"not null"`
	IP           string    `json:"ip"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	SendDate     time.Time `json:"send_date"`
	Reported     bool      `json:"reported" gorm:"not null"`
	ModifiedDate time.Time `json:"modified_date"`
	// RecipientID links this result to the canonical Recipient (person) for
	// the campaign owner (Phase 10a). Zero/NULL when the recipient has no
	// email to key on. Set on creation in PostCampaign.
	RecipientID int64 `json:"-" gorm:"column:recipient_id"`
	BaseRecipient
}

func (r *Result) createEvent(status string, details interface{}) (*Event, error) {
	e := &Event{Email: r.Email, Message: status}
	if details != nil {
		dj, err := json.Marshal(details)
		if err != nil {
			return nil, err
		}
		e.Details = string(dj)
	}
	if err := AddEvent(e, r.CampaignID); err != nil {
		return nil, err
	}
	return e, nil
}

// HandleEmailSent updates a Result to indicate that the email has been
// successfully sent to the remote SMTP server
func (r *Result) HandleEmailSent() error {
	event, err := r.createEvent(EventSent, nil)
	if err != nil {
		return err
	}
	r.SendDate = event.Time
	r.Status = EventSent
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailError updates a Result to indicate that there was an error when
// attempting to send the email to the remote SMTP server.
func (r *Result) HandleEmailError(err error) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = Error
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailBackoff updates a Result to indicate that the email received a
// temporary error and needs to be retried
func (r *Result) HandleEmailBackoff(err error, sendDate time.Time) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = StatusRetry
	r.SendDate = sendDate
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailOpened updates a Result in the case where the recipient opened the
// email.
func (r *Result) HandleEmailOpened(details EventDetails) error {
	event, err := r.createEvent(EventOpened, details)
	if err != nil {
		return err
	}
	// Don't update the status if the user already clicked the link
	// or submitted data to the campaign
	if r.Status == EventClicked || r.Status == EventDataSubmit {
		return nil
	}
	r.Status = EventOpened
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleClickedLink updates a Result in the case where the recipient clicked
// the link in an email.
func (r *Result) HandleClickedLink(details EventDetails) error {
	event, err := r.createEvent(EventClicked, details)
	if err != nil {
		return err
	}
	// Don't update the status if the user has already submitted data via the
	// landing page form.
	if r.Status == EventDataSubmit {
		return nil
	}
	r.Status = EventClicked
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleFormSubmit updates a Result in the case where the recipient submitted
// credentials to the form on a Landing Page.
func (r *Result) HandleFormSubmit(details EventDetails) error {
	event, err := r.createEvent(EventDataSubmit, details)
	if err != nil {
		return err
	}
	r.Status = EventDataSubmit
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailReport updates a Result in the case where they report a simulated
// phishing email using the HTTP handler.
func (r *Result) HandleEmailReport(details EventDetails) error {
	event, err := r.createEvent(EventReported, details)
	if err != nil {
		return err
	}
	r.Reported = true
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// ReportReconcileResult summarizes a manual report-reconciliation run.
type ReportReconcileResult struct {
	Marked          int      `json:"marked"`
	AlreadyReported int      `json:"already_reported"`
	NotFound        int      `json:"not_found"`
	Unmatched       []string `json:"unmatched"`
}

// ReconcileReports credits a campaign's recipients as having reported the
// simulation, from a manually-supplied list of identifiers (each an email
// address or a 7-char result id). This is the "option 3" path for orgs whose
// report mailbox (e.g. a corporate CERT inbox) can't be polled directly: the
// security team confirms who reported and the list is reconciled here, marking
// each matching result Reported (firing the "Email Reported" event so it flows
// into campaign stats and risk scoring). Already-reported and unknown entries
// are reported back rather than erroring.
func ReconcileReports(campaignID int64, uid int64, identifiers []string) (ReportReconcileResult, error) {
	res := ReportReconcileResult{Unmatched: []string{}}
	// Verify the campaign exists and belongs to the operator.
	if err := db.Where("id=? AND user_id=?", campaignID, uid).First(&Campaign{}).Error; err != nil {
		return res, err
	}
	seen := make(map[string]bool)
	for _, raw := range identifiers {
		id := strings.TrimSpace(raw)
		key := strings.ToLower(id)
		if id == "" || seen[key] {
			continue
		}
		seen[key] = true

		r := Result{}
		var q *gorm.DB
		if strings.Contains(id, "@") {
			q = db.Where("campaign_id=? AND user_id=? AND email=?", campaignID, uid, id)
		} else {
			q = db.Where("campaign_id=? AND user_id=? AND r_id=?", campaignID, uid, id)
		}
		if err := q.First(&r).Error; err != nil {
			res.NotFound++
			res.Unmatched = append(res.Unmatched, id)
			continue
		}
		if r.Reported {
			res.AlreadyReported++
			continue
		}
		details := EventDetails{Browser: map[string]string{"source": "manual reconciliation"}}
		if err := r.HandleEmailReport(details); err != nil {
			log.Error(err)
			continue
		}
		res.Marked++
	}
	return res, nil
}

// UpdateGeo updates the latitude and longitude of the result in
// the database given an IP address
func (r *Result) UpdateGeo(addr string) error {
	// Open a connection to the maxmind db
	mmdb, err := maxminddb.Open("static/db/geolite2-city.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = mmdb.Close() }()
	ip := net.ParseIP(addr)
	var city mmCity
	// Get the record
	err = mmdb.Lookup(ip, &city)
	if err != nil {
		return err
	}
	// Update the database with the record information
	r.IP = addr
	r.Latitude = city.GeoPoint.Latitude
	r.Longitude = city.GeoPoint.Longitude
	return db.Save(r).Error
}

func generateResultId() (string, error) {
	const alphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	k := make([]byte, 7)
	for i := range k {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphaNum))))
		if err != nil {
			return "", err
		}
		k[i] = alphaNum[idx.Int64()]
	}
	return string(k), nil
}

// GenerateId generates a unique key to represent the result
// in the database
func (r *Result) GenerateId(tx *gorm.DB) error {
	// Keep trying until we generate a unique key (shouldn't take more than one or two iterations)
	for {
		rid, err := generateResultId()
		if err != nil {
			return err
		}
		r.RID = rid
		err = tx.Table("results").Where("r_id=?", r.RID).First(&Result{}).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}
	}
	return nil
}

// GetResult returns the Result object from the database
// given the ResultId
func GetResult(rid string) (Result, error) {
	r := Result{}
	err := db.Where("r_id=?", rid).First(&r).Error
	return r, err
}

// GetActiveUnreportedResultsByEmail returns the operator's results for the given
// email address that have not been reported yet and belong to a campaign that
// isn't Completed, ordered most-recent first. It backs the IMAP monitor's
// sender-address fallback: when a forwarded report no longer carries the rid,
// the reporter's own address is matched against their in-flight simulations.
//
// A subquery (not a JOIN) restricts to active campaigns on purpose: joining
// campaigns would emit SELECT * across two tables that both have id/user_id/
// status/modified_date columns, and the duplicate names let campaigns.id
// overwrite Result.Id during scan — which would later make HandleEmailReport
// save onto the wrong row.
func GetActiveUnreportedResultsByEmail(uid int64, email string) ([]Result, error) {
	rs := []Result{}
	activeCampaigns := db.Table("campaigns").Select("id").Where("status <> ?", CampaignComplete)
	err := db.Where("user_id = ? AND LOWER(email) = LOWER(?) AND reported = ?", uid, email, false).
		Where("campaign_id IN (?)", activeCampaigns).
		Order("modified_date DESC").
		Find(&rs).Error
	if err != nil {
		log.Error(err)
	}
	return rs, err
}
