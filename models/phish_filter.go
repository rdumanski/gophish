package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rdumanski/gophish/config"
	log "github.com/rdumanski/gophish/logger"
	"gorm.io/gorm"
)

// PhishFilter is the org-wide policy that aggregations apply at query
// time to suppress sandbox-driven opens and clicks (Microsoft Defender
// Safe Links, Proofpoint URL Defense, etc.). Single-row table; the
// row's ID is always 1.
//
// MinClickSeconds: events firing within this many seconds of the
// recipient's SendDate are treated as sandbox pre-scans. 0 disables
// the time check.
//
// SandboxIPs: newline-separated list of CIDR ranges or bare IPs.
// Source IPs falling in any of these are treated as sandbox regardless
// of timing.
type PhishFilter struct {
	ID              int64     `gorm:"primaryKey;column:id" json:"-"`
	MinClickSeconds int       `gorm:"column:min_click_seconds" json:"min_click_seconds"`
	SandboxIPs      string    `gorm:"column:sandbox_ips" json:"sandbox_ips"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName pins the GORM table name (the convention pluralizer would
// produce "phish_filters" otherwise — wrong for our single-row design).
func (PhishFilter) TableName() string { return "phish_filter" }

// ErrInvalidSandboxIP signals a CIDR/IP entry the parser couldn't
// accept. The error wraps the offending raw string so the API handler
// can surface it to the admin.
var ErrInvalidSandboxIP = errors.New("invalid sandbox IP/CIDR")

// GetPhishFilter returns the single policy row, creating an empty one
// if it doesn't yet exist. Side effect: refreshes the in-process
// matcher cache so subsequent aggregation reads see the latest values.
func GetPhishFilter() (PhishFilter, error) {
	pf := PhishFilter{ID: 1}
	err := db.First(&pf, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Lazy-create the row so callers don't have to special-case
		// the empty-DB path.
		pf = PhishFilter{ID: 1, MinClickSeconds: 0, SandboxIPs: "", UpdatedAt: time.Now().UTC()}
		if err := db.Create(&pf).Error; err != nil {
			return pf, err
		}
	} else if err != nil {
		return pf, err
	}
	if err := refreshMatcher(pf); err != nil {
		log.Errorf("phish_filter: matcher refresh failed: %s", err)
	}
	return pf, nil
}

// PutPhishFilter validates and persists the policy, then refreshes
// the in-process matcher cache. Validation rejects negative seconds
// and any unparseable IP/CIDR entry.
func PutPhishFilter(pf *PhishFilter) error {
	if pf.MinClickSeconds < 0 {
		return fmt.Errorf("min_click_seconds must be >= 0")
	}
	// Build a matcher to validate IPs; discard if anything is wrong.
	if _, err := buildMatcher(pf.SandboxIPs, pf.MinClickSeconds); err != nil {
		return err
	}
	pf.ID = 1
	pf.UpdatedAt = time.Now().UTC()
	// GORM's Save does an upsert by primary key — the table has a
	// single row at id=1, so this is the right shape.
	if err := db.Save(pf).Error; err != nil {
		return err
	}
	if err := refreshMatcher(*pf); err != nil {
		log.Errorf("phish_filter: matcher refresh failed: %s", err)
	}
	return nil
}

// SeedPhishFilterFromConfig copies a legacy phish_server.sandbox_filter
// config block into the DB row when the row is empty AND the config
// has values. Idempotent: a populated row blocks future seeds.
//
// Logged at info level so operators can see the migration happened
// once and stop maintaining the config.json values.
func SeedPhishFilterFromConfig(cfg config.PhishFilterConfig) error {
	if cfg.MinClickSeconds == 0 && len(cfg.SandboxIPs) == 0 {
		return nil
	}
	current, err := GetPhishFilter()
	if err != nil {
		return err
	}
	if current.MinClickSeconds != 0 || current.SandboxIPs != "" {
		return nil
	}
	current.MinClickSeconds = cfg.MinClickSeconds
	current.SandboxIPs = strings.Join(cfg.SandboxIPs, "\n")
	if err := PutPhishFilter(&current); err != nil {
		return err
	}
	log.Infof("phish_filter: seeded DB row from config.json — runtime source is now DB; config.json sandbox_filter values are no longer read")
	return nil
}

// --- matcher (relocated from controllers/phish.go) ---------------------------

// sandboxMatcher classifies an event as sandbox-driven based on
// (a) elapsed time since the recipient's SendDate vs. a configured
// floor and (b) whether the event's source IP falls into any of a
// configured set of CIDR ranges. A nil matcher treats all events as
// non-sandbox.
type sandboxMatcher struct {
	minClickDuration time.Duration
	networks         []*net.IPNet
}

// matcherCache caches the parsed PhishFilter for query-time use.
// Aggregation paths Load() it without hitting the DB. The cache is
// refreshed by GetPhishFilter / PutPhishFilter so single-process
// deployments propagate changes instantly. Multi-process deployments
// re-read from DB at server startup; mid-runtime changes propagate
// the next time the admin server restarts (acceptable for an
// org-wide security policy).
var matcherCache atomic.Pointer[sandboxMatcher]

// buildMatcher parses the operator-supplied IP list (newline-separated
// CIDRs or bare IPs) into the runtime form. Bare IPs are promoted to
// /32 (IPv4) or /128 (IPv6) so the rest of the parser is uniform.
// Empty lines are skipped. Returns an error wrapping
// ErrInvalidSandboxIP for the first entry that fails to parse.
func buildMatcher(rawList string, minSeconds int) (*sandboxMatcher, error) {
	var nets []*net.IPNet
	for _, line := range strings.Split(rawList, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			ip := net.ParseIP(s)
			if ip == nil {
				return nil, fmt.Errorf("%w: %q", ErrInvalidSandboxIP, line)
			}
			if ip.To4() != nil {
				s = s + "/32"
			} else {
				s = s + "/128"
			}
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %v", ErrInvalidSandboxIP, line, err)
		}
		nets = append(nets, n)
	}
	return &sandboxMatcher{
		minClickDuration: time.Duration(minSeconds) * time.Second,
		networks:         nets,
	}, nil
}

// refreshMatcher rebuilds the cached matcher from a freshly-loaded
// PhishFilter. Returns an error from buildMatcher; the caller logs
// but continues — bad cache state is preferable to crashing the
// report path.
func refreshMatcher(pf PhishFilter) error {
	m, err := buildMatcher(pf.SandboxIPs, pf.MinClickSeconds)
	if err != nil {
		matcherCache.Store(nil)
		return err
	}
	matcherCache.Store(m)
	return nil
}

// currentMatcher returns the latest cached matcher, or nil if no
// PhishFilter has been loaded yet (e.g. fresh process before
// GetPhishFilter has been called). Nil is filter-off.
func currentMatcher() *sandboxMatcher { return matcherCache.Load() }

// classify returns (true, reason) if the event-with-sendDate looks
// like a sandbox pre-scan. Used by the Filtered helper.
func (m *sandboxMatcher) classify(eventTime, sendDate time.Time, ip string) (bool, string) {
	if m == nil {
		return false, ""
	}
	if m.minClickDuration > 0 && !sendDate.IsZero() && eventTime.Sub(sendDate) < m.minClickDuration {
		return true, "min_click_seconds"
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		for _, n := range m.networks {
			if n.Contains(parsed) {
				return true, "sandbox_ip:" + n.String()
			}
		}
	}
	return false, ""
}

// Filtered returns (true, reason) if the given event (an EventClicked
// or EventOpened, typically) should be suppressed from campaign
// summary aggregations under the current org-wide policy. The reason
// string ("min_click_seconds" or "sandbox_ip:<cidr>") is suitable for
// rendering as a per-event audit badge in the timeline.
func Filtered(e Event, sendDate time.Time) (bool, string) {
	m := currentMatcher()
	if m == nil {
		return false, ""
	}
	ip := extractEventIP(e)
	return m.classify(e.Time, sendDate, ip)
}

// extractEventIP pulls the client address from an event's Details
// (set by setupContext in controllers/phish.go via
// EventDetails.Browser["address"]). Returns "" if the details aren't
// JSON-encoded or don't contain an address.
func extractEventIP(e Event) string {
	if e.Details == "" {
		return ""
	}
	var d EventDetails
	if err := json.Unmarshal([]byte(e.Details), &d); err != nil {
		return ""
	}
	return d.Browser["address"]
}
