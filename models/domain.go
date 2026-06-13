package models

import (
	"errors"
	"regexp"
	"strings"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// Domain roles.
const (
	DomainSending = "sending"
	DomainLanding = "landing"
	DomainBoth    = "both"
)

// Domain is a look-alike domain we own, registered so it can be selected when
// building campaigns (landing host) and sending profiles (sending identity)
// and health-checked. v1 does not generate DKIM keys — the mail relay owns the
// keypair and signs; we record the selector and verify the published record.
type Domain struct {
	Id           int64     `json:"id" gorm:"primaryKey;column:id"`
	UserID       int64     `json:"-" gorm:"column:user_id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	DKIMSelector string    `json:"dkim_selector" gorm:"column:dkim_selector"`
	LastChecked  time.Time `json:"last_checked"`
	LandingOK    bool      `json:"landing_ok" gorm:"column:landing_ok"`
	SPFOK        bool      `json:"spf_ok" gorm:"column:spf_ok"`
	DKIMOK       bool      `json:"dkim_ok" gorm:"column:dkim_ok"`
	DMARCOK      bool      `json:"dmarc_ok" gorm:"column:dmarc_ok"`
	Status       string    `json:"status"`
	ModifiedDate time.Time `json:"modified_date"`
	// Records is the computed list of DNS records the operator should publish;
	// attached on read, never stored.
	Records []DNSRecord `json:"records" gorm:"-"`
}

// DNSRecord is one suggested DNS record to publish for a domain.
type DNSRecord struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
	Note  string `json:"note"`
}

// Domain validation errors.
var (
	ErrDomainNameNotSpecified = errors.New("domain name not specified")
	ErrDomainNameInvalid      = errors.New("domain name is not a valid hostname")
	ErrDomainRoleInvalid      = errors.New(`domain role must be "sending", "landing", or "both"`)
)

// hostnameRegex is a permissive RFC-1123 hostname check (labels of a-z0-9-,
// at least one dot). Good enough to reject obvious garbage / URLs.
var hostnameRegex = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

// IsLanding reports whether the domain serves the landing/links host.
func (d *Domain) IsLanding() bool { return d.Role == DomainLanding || d.Role == DomainBoth }

// IsSending reports whether the domain is used as the email sending identity.
func (d *Domain) IsSending() bool { return d.Role == DomainSending || d.Role == DomainBoth }

// Validate checks the domain fields and normalizes the name.
func (d *Domain) Validate() error {
	d.Name = strings.ToLower(strings.TrimSpace(d.Name))
	if d.Name == "" {
		return ErrDomainNameNotSpecified
	}
	if !hostnameRegex.MatchString(d.Name) {
		return ErrDomainNameInvalid
	}
	switch d.Role {
	case DomainSending, DomainLanding, DomainBoth:
	case "":
		d.Role = DomainBoth
	default:
		return ErrDomainRoleInvalid
	}
	return nil
}

// SuggestedRecords returns the DNS records the operator should publish for this
// domain's role(s). Advisory text — actual values (server IP, DKIM key) come
// from the operator's infrastructure / mail relay.
func (d *Domain) SuggestedRecords() []DNSRecord {
	recs := []DNSRecord{}
	if d.IsLanding() {
		recs = append(recs, DNSRecord{
			Type:  "A / CNAME",
			Host:  d.Name,
			Value: "<your phishing server or TLS proxy>",
			Note:  "Point the domain at your phishing server (or the reverse proxy terminating HTTPS — e.g. Caddy auto-issuing Let's Encrypt).",
		})
	}
	if d.IsSending() {
		sel := d.DKIMSelector
		if sel == "" {
			sel = "<selector>"
		}
		recs = append(recs,
			DNSRecord{Type: "TXT (SPF)", Host: d.Name, Value: "v=spf1 include:<your-mail-relay> ~all", Note: "Authorize your sending relay."},
			DNSRecord{Type: "TXT (DKIM)", Host: sel + "._domainkey." + d.Name, Value: "v=DKIM1; k=rsa; p=<public key from your relay>", Note: "Publish the public key your relay signs with."},
			DNSRecord{Type: "TXT (DMARC)", Host: "_dmarc." + d.Name, Value: "v=DMARC1; p=none; rua=mailto:dmarc@" + d.Name, Note: "Start with p=none while testing."},
		)
	}
	return recs
}

// --- Pure health-check parsers (unit-tested; the live lookups live in the
// `domain` package which feeds these). ---

// SPFPresent reports whether any TXT record is an SPF record.
func SPFPresent(txts []string) bool {
	for _, t := range txts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t)), "v=spf1") {
			return true
		}
	}
	return false
}

// DMARCPresent reports whether any TXT record is a DMARC record.
func DMARCPresent(txts []string) bool {
	for _, t := range txts {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(t)), "v=dmarc1") {
			return true
		}
	}
	return false
}

// DKIMPresent reports whether any TXT record is a well-formed DKIM key record.
func DKIMPresent(txts []string) bool {
	for _, t := range txts {
		l := strings.ToLower(t)
		if strings.Contains(l, "v=dkim1") && strings.Contains(l, "p=") {
			return true
		}
	}
	return false
}

// RobotsMarkerOK reports whether the body is gophish's phishing-server robots
// response — the always-on, unauthenticated marker the landing probe matches.
func RobotsMarkerOK(body string) bool {
	return strings.Contains(body, "User-agent: *") && strings.Contains(body, "Disallow: /")
}

// --- CRUD ---

// GetDomains returns the operator's domains with suggested records attached.
func GetDomains(uid int64) ([]Domain, error) {
	ds := []Domain{}
	if err := db.Where("user_id=?", uid).Find(&ds).Error; err != nil {
		log.Error(err)
		return ds, err
	}
	for i := range ds {
		ds[i].Records = ds[i].SuggestedRecords()
	}
	return ds, nil
}

// GetDomain returns a single domain with suggested records attached.
func GetDomain(id int64, uid int64) (Domain, error) {
	d := Domain{}
	if err := db.Where("user_id=? and id=?", uid, id).First(&d).Error; err != nil {
		log.Error(err)
		return d, err
	}
	d.Records = d.SuggestedRecords()
	return d, nil
}

// GetDomainByName returns the domain with the given name for a user. A
// gorm.ErrRecordNotFound result is expected by the uniqueness check and not
// logged.
func GetDomainByName(name string, uid int64) (Domain, error) {
	d := Domain{}
	err := db.Where("user_id=? and name=?", uid, strings.ToLower(strings.TrimSpace(name))).First(&d).Error
	return d, err
}

// PostDomain creates a domain after validation.
func PostDomain(d *Domain, uid int64) error {
	if err := d.Validate(); err != nil {
		return err
	}
	d.UserID = uid
	d.ModifiedDate = time.Now().UTC()
	if err := db.Save(d).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PutDomain updates a domain after validation.
func PutDomain(d *Domain, uid int64) error {
	if err := d.Validate(); err != nil {
		return err
	}
	d.UserID = uid
	d.ModifiedDate = time.Now().UTC()
	if err := db.Where("id=?", d.Id).Save(d).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// DeleteDomain deletes a domain by id, scoped to its owner.
func DeleteDomain(id int64, uid int64) error {
	if err := db.Where("user_id=?", uid).Delete(&Domain{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SaveDomainHealth persists the health-check results.
func SaveDomainHealth(d *Domain) error {
	return db.Model(&Domain{}).Where("id=?", d.Id).Updates(map[string]interface{}{
		"last_checked": d.LastChecked,
		"landing_ok":   d.LandingOK,
		"spf_ok":       d.SPFOK,
		"dkim_ok":      d.DKIMOK,
		"dmarc_ok":     d.DMARCOK,
		"status":       d.Status,
	}).Error
}
