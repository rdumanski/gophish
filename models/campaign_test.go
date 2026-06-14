package models

import (
	"fmt"
	"testing"
	"time"

	check "gopkg.in/check.v1"
	"gorm.io/gorm"
)

func (s *ModelsSuite) TestGenerateSendDate(c *check.C) {
	campaign := s.createCampaignDependencies(c)
	// Test that if no launch date is provided, the campaign's creation date
	// is used.
	err := PostCampaign(&campaign, campaign.UserID)
	c.Assert(err, check.Equals, nil)
	c.Assert(campaign.LaunchDate, check.Equals, campaign.CreatedDate)

	// For comparing the dates, we need to fetch the campaign again. This is
	// to solve an issue where the campaign object right now has time down to
	// the microsecond, while in MySQL it's rounded down to the second.
	campaign, _ = GetCampaign(campaign.Id, campaign.UserID)

	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	for _, m := range ms {
		c.Assert(m.SendDate, check.Equals, campaign.CreatedDate)
	}

	// Test that if no send date is provided, all the emails are sent at the
	// campaign's launch date
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	err = PostCampaign(&campaign, campaign.UserID)
	c.Assert(err, check.Equals, nil)

	campaign, _ = GetCampaign(campaign.Id, campaign.UserID)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	for _, m := range ms {
		c.Assert(m.SendDate, check.Equals, campaign.LaunchDate)
	}

	// Finally, test that if a send date is provided, the emails are staggered
	// correctly.
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(2 * time.Minute)
	err = PostCampaign(&campaign, campaign.UserID)
	c.Assert(err, check.Equals, nil)

	campaign, _ = GetCampaign(campaign.Id, campaign.UserID)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	sendingOffset := 2 / float64(len(ms))
	for i, m := range ms {
		expectedOffset := int(sendingOffset * float64(i))
		expectedDate := campaign.LaunchDate.Add(time.Duration(expectedOffset) * time.Minute)
		c.Assert(m.SendDate, check.Equals, expectedDate)
	}
}

func (s *ModelsSuite) TestCampaignDateValidation(c *check.C) {
	campaign := s.createCampaignDependencies(c)
	// If both are zero, then the campaign should start immediately with no
	// send by date
	err := campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the launch date is specified, then the send date is optional
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	err = campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the send date is greater than the launch date, then there's no
	//problem
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(1 * time.Minute)
	err = campaign.Validate()
	c.Assert(err, check.Equals, nil)

	// If the send date is less than the launch date, then there's an issue
	campaign = s.createCampaignDependencies(c)
	campaign.LaunchDate = time.Now().UTC()
	campaign.SendByDate = campaign.LaunchDate.Add(-1 * time.Minute)
	err = campaign.Validate()
	c.Assert(err, check.Equals, ErrInvalidSendByDate)
}

func (s *ModelsSuite) TestLaunchCampaignMaillogStatus(c *check.C) {
	// For the first test, ensure that campaigns created with the zero date
	// (and therefore are set to launch immediately) have maillogs that are
	// locked to prevent race conditions.
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, true)
	}

	// Next, verify that campaigns scheduled in the future do not lock the
	// maillogs so that they can be picked up by the background worker.
	campaign = s.createCampaignDependencies(c)
	campaign.Name = "New Campaign"
	campaign.LaunchDate = time.Now().Add(1 * time.Hour)
	c.Assert(PostCampaign(&campaign, campaign.UserID), check.Equals, nil)
	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	for _, m := range ms {
		c.Assert(m.Processing, check.Equals, false)
	}
}

func (s *ModelsSuite) TestDeleteCampaignAlsoDeletesMailLogs(c *check.C) {
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = DeleteCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestCompleteCampaignAlsoDeletesMailLogs(c *check.C) {
	campaign := s.createCampaign(c)
	ms, err := GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, len(campaign.Results))

	err = CompleteCampaign(campaign.Id, campaign.UserID)
	c.Assert(err, check.Equals, nil)

	ms, err = GetMailLogsByCampaign(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ms), check.Equals, 0)
}

func (s *ModelsSuite) TestCampaignGetResults(c *check.C) {
	campaign := s.createCampaign(c)
	got, err := GetCampaign(campaign.Id, campaign.UserID)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(campaign.Results), check.Equals, len(got.Results))
}

// TestCampaignStatsCountsReported pins the GORM v2 condition-accumulation fix
// in getCampaignStats. The reused query object meant EmailReported was ANDed
// with the prior status='Submitted Data' filter and read 0 even when
// recipients had reported. Marking results reported must be reflected in the
// stats (which drive the dashboard "Reported" metric and risk scoring).
func (s *ModelsSuite) TestCampaignStatsCountsReported(c *check.C) {
	campaign := s.createCampaign(c)
	c.Assert(len(campaign.Results) >= 2, check.Equals, true)

	for i := 0; i < 2; i++ {
		c.Assert(campaign.Results[i].HandleEmailReport(EventDetails{}), check.Equals, nil)
	}

	stats, err := getCampaignStats(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(stats.Total, check.Equals, int64(len(campaign.Results)))
	c.Assert(stats.EmailReported, check.Equals, int64(2))
}

// TestGetCampaignResults pins the GORM v2 strict-scan fix: CampaignResults
// is a DTO whose Results/Events slices are loaded by separate follow-up
// queries, so they must be tagged gorm:"-". Without the tag the initial
// db.Table("campaigns").First(&cr) scan errors with "invalid field ...
// define a valid foreign key", which surfaced as a 404 "Campaign not
// found!" on the View Results page.
func (s *ModelsSuite) TestGetCampaignResults(c *check.C) {
	campaign := s.createCampaign(c)
	cr, err := GetCampaignResults(campaign.Id, campaign.UserID)
	c.Assert(err, check.Equals, nil)
	c.Assert(cr.Id, check.Equals, campaign.Id)
	c.Assert(cr.Name, check.Equals, campaign.Name)
	c.Assert(len(cr.Results), check.Equals, len(campaign.Results))
}

// TestCampaignStatsAppliesPhishFilterRetroactively pins down the
// defining property of Phase 7c.2: the campaign summary reads the
// CURRENT phish_filter policy at query time, so a click recorded
// under one policy is reclassified the next time the summary runs
// under a tighter policy. Without this retroactive behaviour the UI
// can't show "did changing min_click_seconds reclassify my
// historical campaigns?", which was the whole point of the rewrite.
func (s *ModelsSuite) TestCampaignStatsAppliesPhishFilterRetroactively(c *check.C) {
	campaign := s.createCampaign(c)
	c.Assert(len(campaign.Results) > 0, check.Equals, true)

	// Pick the first recipient and record a click 3 seconds after their
	// scheduled send. Insert directly into the events table so the
	// recorded Event.Time is deterministic — the stock HandleClickedLink
	// path uses time.Now(), which races with the assertions below.
	r := campaign.Results[0]
	clickTime := r.SendDate.Add(3 * time.Second)
	clickEvent := Event{
		CampaignID: campaign.Id,
		Email:      r.Email,
		Time:       clickTime,
		Message:    EventClicked,
		Details:    "",
	}
	c.Assert(db.Create(&clickEvent).Error, check.Equals, nil)
	// Roll the Result.Status forward too so the event-table walk
	// agrees with the column-based aggregations on EmailsSent.
	r.Status = EventClicked
	r.ModifiedDate = clickTime
	c.Assert(db.Save(&r).Error, check.Equals, nil)

	// Filter off — the click counts.
	c.Assert(PutPhishFilter(&PhishFilter{MinClickSeconds: 0}), check.Equals, nil)
	stats, err := getCampaignStats(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(stats.ClickedLink, check.Equals, int64(1))

	// Tighten the policy to 10s — the same recorded click is now
	// reclassified as a sandbox pre-scan and falls out of the count.
	c.Assert(PutPhishFilter(&PhishFilter{MinClickSeconds: 10}), check.Equals, nil)
	stats, err = getCampaignStats(campaign.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(stats.ClickedLink, check.Equals, int64(0))
}

func setupCampaignDependencies(b *testing.B, size int) {
	group := Group{Name: "Test Group"}
	// Create a large group of 5000 members
	for i := 0; i < size; i++ {
		group.Targets = append(group.Targets, Target{BaseRecipient: BaseRecipient{Email: fmt.Sprintf("test%d@example.com", i), FirstName: "User", LastName: fmt.Sprintf("%d", i)}})
	}
	group.UserID = 1
	err := PostGroup(&group)
	if err != nil {
		b.Fatalf("error posting group: %v", err)
	}

	// Add a template
	template := Template{Name: "Test Template"}
	template.Subject = "{{.RID}} - Subject"
	template.Text = "{{.RID}} - Text"
	template.HTML = "{{.RID}} - HTML"
	template.UserID = 1
	err = PostTemplate(&template)
	if err != nil {
		b.Fatalf("error posting template: %v", err)
	}

	// Add a landing page
	p := Page{Name: "Test Page"}
	p.HTML = "<html>Test</html>"
	p.UserID = 1
	err = PostPage(&p)
	if err != nil {
		b.Fatalf("error posting page: %v", err)
	}

	// Add a sending profile
	smtp := SMTP{Name: "Test Page"}
	smtp.UserID = 1
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	err = PostSMTP(&smtp)
	if err != nil {
		b.Fatalf("error posting smtp: %v", err)
	}
}

// setupCampaign sets up the campaign dependencies as well as posting the
// actual campaign
func setupCampaign(b *testing.B, size int) Campaign {
	setupCampaignDependencies(b, size)
	campaign := Campaign{Name: "Test campaign"}
	campaign.UserID = 1
	campaign.Template = Template{Name: "Test Template"}
	campaign.Page = Page{Name: "Test Page"}
	campaign.SMTP = SMTP{Name: "Test Page"}
	campaign.Groups = []Group{Group{Name: "Test Group"}}
	PostCampaign(&campaign, 1)
	return campaign
}

func BenchmarkCampaign100(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserID = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		gdb := db.Session(&gorm.Session{AllowGlobalUpdate: true})
		gdb.Delete(&Result{})
		gdb.Delete(&MailLog{})
		gdb.Delete(&Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkCampaign1000(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserID = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		gdb := db.Session(&gorm.Session{AllowGlobalUpdate: true})
		gdb.Delete(&Result{})
		gdb.Delete(&MailLog{})
		gdb.Delete(&Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkCampaign10000(b *testing.B) {
	setupBenchmark(b)
	setupCampaignDependencies(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		campaign := Campaign{Name: "Test campaign"}
		campaign.UserID = 1
		campaign.Template = Template{Name: "Test Template"}
		campaign.Page = Page{Name: "Test Page"}
		campaign.SMTP = SMTP{Name: "Test Page"}
		campaign.Groups = []Group{Group{Name: "Test Group"}}

		b.StartTimer()
		err := PostCampaign(&campaign, 1)
		if err != nil {
			b.Fatalf("error posting campaign: %v", err)
		}
		b.StopTimer()
		gdb := db.Session(&gorm.Session{AllowGlobalUpdate: true})
		gdb.Delete(&Result{})
		gdb.Delete(&MailLog{})
		gdb.Delete(&Campaign{})
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign100(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserID)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign1000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserID)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign5000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserID)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}

func BenchmarkGetCampaign10000(b *testing.B) {
	setupBenchmark(b)
	campaign := setupCampaign(b, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetCampaign(campaign.Id, campaign.UserID)
		if err != nil {
			b.Fatalf("error getting campaign: %v", err)
		}
	}
	tearDownBenchmark(b)
}
