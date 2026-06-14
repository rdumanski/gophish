package models

import (
	"time"

	check "gopkg.in/check.v1"
)

// fullPeriod is a wide window that includes "now" for tests that don't care
// about period edges.
func fullPeriod() (time.Time, time.Time) {
	return time.Now().UTC().AddDate(-1, 0, 0), time.Now().UTC().AddDate(0, 0, 1)
}

func (s *ModelsSuite) TestBucketRisk(c *check.C) {
	// Boundaries: <40 low, <70 medium, else high.
	rp := bucketRisk([]RiskScore{{Score: 70}, {Score: 69}, {Score: 40}, {Score: 39}})
	c.Assert(rp.Low, check.Equals, int64(1))    // 39
	c.Assert(rp.Medium, check.Equals, int64(2)) // 40, 69
	c.Assert(rp.High, check.Equals, int64(1))   // 70
	c.Assert(rp.Average, check.Equals, 54.5)
	c.Assert(len(rp.Top), check.Equals, 4)

	// Empty input: no panic, zero values, non-nil Top.
	empty := bucketRisk(nil)
	c.Assert(empty.Average, check.Equals, float64(0))
	c.Assert(len(empty.Top), check.Equals, 0)
}

func (s *ModelsSuite) TestComplianceReportEmpty(c *check.C) {
	start, end := fullPeriod()
	rep, err := GetComplianceReport(1, start, end)
	c.Assert(err, check.Equals, nil)
	c.Assert(rep.Population, check.Equals, int64(0))
	c.Assert(rep.Phishing.Campaigns, check.Equals, int64(0))
	c.Assert(rep.Phishing.ClickRate, check.Equals, float64(0)) // no divide-by-zero
	c.Assert(rep.Training.CompletionRate, check.Equals, float64(0))
	c.Assert(len(rep.Groups), check.Equals, 0)
}

// TestComplianceReportPeriodFiltering pins that campaigns launched and
// enrollments assigned outside the window are excluded.
func (s *ModelsSuite) TestComplianceReportPeriodFiltering(c *check.C) {
	// In-period campaign (launch defaults to ~now).
	s.createCampaign(c)
	// Out-of-period campaign: backdate its launch_date two years.
	old := s.createCampaign(c)
	twoYearsAgo := time.Now().UTC().AddDate(-2, 0, 0)
	c.Assert(db.Model(&Campaign{}).Where("id=?", old.Id).
		Update("launch_date", twoYearsAgo).Error, check.Equals, nil)

	// In-period enrollment.
	mod := s.makeModule(c)
	_, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "now@example.com"}, mod.Id)
	c.Assert(err, check.Equals, nil)
	// Out-of-period enrollment: backdate assigned_date.
	e2, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "old@example.com"}, mod.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(db.Model(&Enrollment{}).Where("id=?", e2.Id).
		Update("assigned_date", twoYearsAgo).Error, check.Equals, nil)

	start, end := fullPeriod()
	rep, err := GetComplianceReport(1, start, end)
	c.Assert(err, check.Equals, nil)
	c.Assert(rep.Phishing.Campaigns, check.Equals, int64(1)) // only the in-period one
	c.Assert(rep.Training.Assigned, check.Equals, int64(1))  // only the in-period cohort
}

// TestComplianceGroupRollup pins the per-group breakdown, including email
// normalization (a clicked result stored upper-case still matches a lower-case
// group member) and the training-completion bridge via recipient_id.
func (s *ModelsSuite) TestComplianceGroupRollup(c *check.C) {
	campaign := s.createCampaign(c) // group "Test Group", members test1..4@example.com
	c.Assert(len(campaign.Results), check.Equals, 4)

	// test1 clicked — store the result email UPPER-CASE to exercise normalization.
	c.Assert(db.Model(&Result{}).Where("campaign_id=? AND email=?", campaign.Id, "test1@example.com").
		Updates(map[string]interface{}{"status": EventClicked, "email": "TEST1@EXAMPLE.COM"}).Error,
		check.Equals, nil)
	// test2 reported.
	c.Assert(db.Model(&Result{}).Where("campaign_id=? AND email=?", campaign.Id, "test2@example.com").
		Update("reported", true).Error, check.Equals, nil)
	// test3 completed training (recipient is shared with the campaign via upsert).
	mod := s.makeModule(c)
	e, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "test3@example.com"}, mod.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(e.MarkCompleted(), check.Equals, nil)

	start, end := fullPeriod()
	rep, err := GetComplianceReport(1, start, end)
	c.Assert(err, check.Equals, nil)

	var row *GroupComplianceRow
	for i := range rep.Groups {
		if rep.Groups[i].Name == "Test Group" {
			row = &rep.Groups[i]
		}
	}
	c.Assert(row, check.NotNil)
	c.Assert(row.Members, check.Equals, int64(4))
	c.Assert(row.ClickPct, check.Equals, float64(25))   // 1/4, matched despite upper-case
	c.Assert(row.ReportPct, check.Equals, float64(25))  // 1/4
	c.Assert(row.TrainedPct, check.Equals, float64(25)) // 1/4
}
