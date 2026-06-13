// Package domain runs the live health check for a custom-domains registry
// entry: an HTTPS reachability probe for the landing role and SPF/DKIM/DMARC
// TXT lookups for the sending role. The TXT/cert parsing lives in package
// models (unit-tested); this package does the network I/O.
package domain

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rdumanski/gophish/dialer"
	"github.com/rdumanski/gophish/models"
)

// Check runs the health check for a domain, updates its health fields, and
// persists them. Outbound HTTP goes through the restricted dialer (the same
// SSRF guard ImportSite/webhooks use), so a probe against a loopback/internal
// host is blocked unless allow_internal_hosts is configured.
func Check(d *models.Domain) {
	now := time.Now().UTC()
	parts := []string{}

	if d.IsLanding() {
		ok, msg := probeLanding(d.Name)
		d.LandingOK = ok
		parts = append(parts, "landing: "+msg)
	}
	if d.IsSending() {
		spf, _ := net.LookupTXT(d.Name)
		d.SPFOK = models.SPFPresent(spf)

		if d.DKIMSelector != "" {
			dk, _ := net.LookupTXT(d.DKIMSelector + "._domainkey." + d.Name)
			d.DKIMOK = models.DKIMPresent(dk)
		} else {
			d.DKIMOK = false
		}

		dm, _ := net.LookupTXT("_dmarc." + d.Name)
		d.DMARCOK = models.DMARCPresent(dm)

		dkimNote := "found"
		if d.DKIMSelector == "" {
			dkimNote = "no selector set"
		} else if !d.DKIMOK {
			dkimNote = "missing"
		}
		parts = append(parts, fmt.Sprintf("SPF: %s | DKIM: %s | DMARC: %s",
			yesno(d.SPFOK), dkimNote, yesno(d.DMARCOK)))
	}

	d.LastChecked = now
	d.Status = strings.Join(parts, "  ||  ")
	_ = models.SaveDomainHealth(d)
}

func yesno(b bool) string {
	if b {
		return "ok"
	}
	return "missing"
}

// probeLanding does an HTTPS GET of the domain's /robots.txt and confirms it's
// served by a gophish phishing server. This validates DNS + TLS + "wired to us"
// in one shot and is reverse-proxy compatible (we don't compare resolved IPs).
// The cert IS verified (valid HTTPS is the point), unlike ImportSite.
func probeLanding(name string) (bool, string) {
	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{DialContext: dialer.Dialer().DialContext},
		// Don't follow redirects — we want the phishing server's own response.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("https://" + name + "/robots.txt")
	if err != nil {
		return false, "not reachable over HTTPS (" + err.Error() + ")"
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if models.RobotsMarkerOK(string(body)) {
		return true, "reachable over HTTPS and serving the phishing server"
	}
	return false, "reachable over HTTPS but it is not a gophish phishing server"
}
