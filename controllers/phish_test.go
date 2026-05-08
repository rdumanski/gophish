package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rdumanski/gophish/config"
	"github.com/rdumanski/gophish/models"
)

func getFirstCampaign(t *testing.T) models.Campaign {
	campaigns, err := models.GetCampaigns(1)
	if err != nil {
		t.Fatalf("error getting first campaign from database: %v", err)
	}
	return campaigns[0]
}

func getFirstEmailRequest(t *testing.T) models.EmailRequest {
	campaign := getFirstCampaign(t)
	req := models.EmailRequest{
		TemplateID:    campaign.TemplateID,
		Template:      campaign.Template,
		PageID:        campaign.PageID,
		Page:          campaign.Page,
		URL:           "http://localhost.localdomain",
		UserID:        1,
		BaseRecipient: campaign.Results[0].BaseRecipient,
		SMTP:          campaign.SMTP,
		FromAddress:   campaign.SMTP.FromAddress,
	}
	err := models.PostEmailRequest(&req)
	if err != nil {
		t.Fatalf("error creating email request: %v", err)
	}
	return req
}

func openEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /track endpoint: %v", err)
	}
	expected, err := os.ReadFile("static/images/pixel.png")
	if err != nil {
		t.Fatalf("error reading local transparent pixel: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected tracking pixel data received. expected %#v got %#v", expected, got)
	}
}

func openEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}
}

func reportedEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNoContent
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func reportEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func clickLink(t *testing.T, ctx *testContext, rid string, expectedHTML string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading payload from / endpoint response: %v", err)
	}
	if !bytes.Equal(got, []byte(expectedHTML)) {
		t.Fatalf("invalid response received from / endpoint. expected %s got %s", got, expectedHTML)
	}
}

func clickLink404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
}

func transparencyRequest(t *testing.T, ctx *testContext, r models.Result, rid, path string) {
	resp, err := http.Get(fmt.Sprintf("%s%s?%s=%s", ctx.phishServer.URL, path, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting %s endpoint: %v", path, err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
	tr := &TransparencyResponse{}
	err = json.NewDecoder(resp.Body).Decode(tr)
	if err != nil {
		t.Fatalf("error unmarshaling transparency request: %v", err)
	}
	expectedResponse := &TransparencyResponse{
		ContactAddress: ctx.config.ContactAddress,
		SendDate:       r.SendDate,
		Server:         config.ServerName,
	}
	if !reflect.DeepEqual(tr, expectedResponse) {
		t.Fatalf("unexpected transparency response received. expected %v got %v", expectedResponse, tr)
	}
}

func TestOpenedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RID)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
	if lastEvent.Message != models.EventOpened {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventOpened)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestReportedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	reportedEmail(t, ctx, result.RID)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]

	if result.Reported != true {
		t.Fatalf("unexpected result report status received. expected %v got %v", true, result.Reported)
	}
	if lastEvent.Message != models.EventReported {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventReported)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestClickedPhishingLinkAfterOpen(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RID)
	clickLink(t, ctx, result.RID, campaign.Page.HTML)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventClicked {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventClicked, result.Status)
	}
	if lastEvent.Message != models.EventClicked {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventClicked)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestNoRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/track", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}

	resp, err = http.Get(ctx.phishServer.URL)
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	got = resp.StatusCode
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
}

func TestInvalidRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	rid := "XXXXXXXXXX"
	openEmail404(t, ctx, rid)
	clickLink404(t, ctx, rid)
	reportEmail404(t, ctx, rid)
}

func TestCompletedCampaignClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RID)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}

	models.CompleteCampaign(campaign.Id, 1)
	openEmail404(t, ctx, result.RID)
	clickLink404(t, ctx, result.RID)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
}

func TestRobotsHandler(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/robots.txt", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /robots.txt endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusOK
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expected := []byte("User-agent: *\nDisallow: /\n")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /robots.txt endpoint: %v", err)
	}
	if !bytes.Equal(body, expected) {
		t.Fatalf("invalid robots.txt response received. expected %s got %s", expected, body)
	}
}

func TestInvalidPreviewID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	bogusRID := fmt.Sprintf("%sbogus", models.PreviewPrefix)
	openEmail404(t, ctx, bogusRID)
	clickLink404(t, ctx, bogusRID)
	reportEmail404(t, ctx, bogusRID)
}

func TestPreviewTrack(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	openEmail(t, ctx, req.RID)
}

func TestPreviewClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	clickLink(t, ctx, req.RID, req.Page.HTML)
}

func TestInvalidTransparencyRequest(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	bogusRID := fmt.Sprintf("bogus%s", TransparencySuffix)
	openEmail404(t, ctx, bogusRID)
	clickLink404(t, ctx, bogusRID)
	reportEmail404(t, ctx, bogusRID)
}

func TestTransparencyRequest(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	rid := fmt.Sprintf("%s%s", result.RID, TransparencySuffix)
	transparencyRequest(t, ctx, result, rid, "/")
	transparencyRequest(t, ctx, result, rid, "/track")
	transparencyRequest(t, ctx, result, rid, "/report")

	// And check with the URL encoded version of a +
	rid = fmt.Sprintf("%s%s", result.RID, "%2b")
	transparencyRequest(t, ctx, result, rid, "/")
	transparencyRequest(t, ctx, result, rid, "/track")
	transparencyRequest(t, ctx, result, rid, "/report")
}

func TestRedirectTemplating(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	p := models.Page{
		Name:        "Redirect Page",
		HTML:        "<html>Test</html>",
		UserID:      1,
		RedirectURL: "http://example.com/{{.RID}}",
	}
	err := models.PostPage(&p)
	if err != nil {
		t.Fatalf("error posting new page: %v", err)
	}
	smtp, _ := models.GetSMTP(1, 1)
	template, _ := models.GetTemplate(1, 1)
	group, _ := models.GetGroup(1, 1)

	campaign := models.Campaign{Name: "Redirect campaign"}
	campaign.UserID = 1
	campaign.Template = template
	campaign.Page = p
	campaign.SMTP = smtp
	campaign.Groups = []models.Group{group}
	err = models.PostCampaign(&campaign, campaign.UserID)
	if err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	result := campaign.Results[0]
	resp, err := client.PostForm(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, result.RID), url.Values{"username": {"test"}, "password": {"test"}})
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusFound
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expectedURL := fmt.Sprintf("http://example.com/%s", result.RID)
	gotURL, err := resp.Location()
	if err != nil {
		t.Fatalf("error getting Location header from response: %v", err)
	}
	if gotURL.String() != expectedURL {
		t.Fatalf("invalid redirect received. expected %s got %s", expectedURL, gotURL)
	}
}

// --- sandboxMatcher tests ---------------------------------------------------

func TestNewSandboxMatcherEmptyConfig(t *testing.T) {
	m, err := newSandboxMatcher(config.PhishFilterConfig{})
	if err != nil {
		t.Fatalf("empty config: unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("matcher should be non-nil for zero-value config")
	}
	if filtered, _ := m.classify(time.Now().Add(-time.Hour), "203.0.113.5"); filtered {
		t.Errorf("zero-value config should treat everything as non-sandbox")
	}
}

func TestNewSandboxMatcherInvalidIP(t *testing.T) {
	cases := []string{"not-an-ip", "256.256.256.256", "1.2.3.4/zz"}
	for _, raw := range cases {
		_, err := newSandboxMatcher(config.PhishFilterConfig{SandboxIPs: []string{raw}})
		if err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestSandboxMatcherTimeThreshold(t *testing.T) {
	m, err := newSandboxMatcher(config.PhishFilterConfig{MinClickSeconds: 5})
	if err != nil {
		t.Fatalf("newSandboxMatcher: %v", err)
	}
	// SendDate 2 seconds ago, threshold 5s → filter.
	if filtered, reason := m.classify(time.Now().Add(-2*time.Second), "203.0.113.5"); !filtered {
		t.Errorf("expected filter for 2s elapsed under 5s threshold, got non-filter")
	} else if reason != "min_click_seconds" {
		t.Errorf("reason: got %q, want %q", reason, "min_click_seconds")
	}
	// SendDate 10 seconds ago → above threshold, do not filter.
	if filtered, _ := m.classify(time.Now().Add(-10*time.Second), "203.0.113.5"); filtered {
		t.Errorf("expected no filter for 10s elapsed over 5s threshold")
	}
	// Zero SendDate (race / weird state) should not filter.
	if filtered, _ := m.classify(time.Time{}, "203.0.113.5"); filtered {
		t.Errorf("zero SendDate should not trigger time filter")
	}
}

func TestSandboxMatcherIPCIDR(t *testing.T) {
	m, err := newSandboxMatcher(config.PhishFilterConfig{
		SandboxIPs: []string{"192.0.2.0/24", "203.0.113.10", "2001:db8::/32"},
	})
	if err != nil {
		t.Fatalf("newSandboxMatcher: %v", err)
	}
	old := time.Now().Add(-time.Hour) // outside any time threshold; isolate the IP test
	cases := []struct {
		ip       string
		filtered bool
	}{
		{"192.0.2.5", true},     // CIDR match
		{"192.0.2.255", true},   // CIDR match (boundary)
		{"192.0.3.5", false},    // outside CIDR
		{"203.0.113.10", true},  // bare IP promoted to /32
		{"203.0.113.11", false}, // adjacent host outside /32
		{"2001:db8::1", true},   // IPv6 CIDR match
		{"2001:db9::1", false},  // outside IPv6 CIDR
		{"not-an-ip", false},    // unparseable → never matches
	}
	for _, tc := range cases {
		filtered, reason := m.classify(old, tc.ip)
		if filtered != tc.filtered {
			t.Errorf("classify(%q): got filtered=%v, want %v", tc.ip, filtered, tc.filtered)
		}
		if filtered && reason == "" {
			t.Errorf("classify(%q): filtered=true but empty reason", tc.ip)
		}
	}
}

func TestSandboxMatcherNilReceiver(t *testing.T) {
	// A nil matcher (sandbox filter wholly disabled) must treat
	// everything as non-sandbox without panicking.
	var m *sandboxMatcher
	if filtered, _ := m.classify(time.Now(), "203.0.113.5"); filtered {
		t.Errorf("nil matcher should never filter")
	}
}

func TestSandboxMatcherTimeFilterWinsBeforeIP(t *testing.T) {
	// When both filters would catch the request, the function returns
	// the time reason first — verifies caller doesn't see "sandbox_ip:..."
	// for fast clicks from non-sandbox addresses.
	m, err := newSandboxMatcher(config.PhishFilterConfig{
		MinClickSeconds: 5,
		SandboxIPs:      []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("newSandboxMatcher: %v", err)
	}
	filtered, reason := m.classify(time.Now().Add(-time.Second), "203.0.113.10")
	if !filtered || reason != "min_click_seconds" {
		t.Errorf("expected min_click_seconds reason, got filtered=%v reason=%q", filtered, reason)
	}
}
