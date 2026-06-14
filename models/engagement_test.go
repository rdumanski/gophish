package models

import (
	check "gopkg.in/check.v1"
)

func hasBadge(badges []string, b string) bool {
	for _, x := range badges {
		if x == b {
			return true
		}
	}
	return false
}

// TestComputeEngagement pins the score direction and badge assignment.
func (s *ModelsSuite) TestComputeEngagement(c *check.C) {
	// Perfect reporter: reported every sim, completed all training → high + badges.
	hi, hb := computeEngagement(RiskScore{Sims: 4, Reported: 4, TrainingAssigned: 1, TrainingCompleted: 1})
	c.Assert(hi >= 90, check.Equals, true)
	c.Assert(hasBadge(hb, "perfect_reporter"), check.Equals, true)
	c.Assert(hasBadge(hb, "sharp_eye"), check.Equals, true) // never clicked
	c.Assert(hasBadge(hb, "scholar"), check.Equals, true)
	c.Assert(hasBadge(hb, "vigilant"), check.Equals, true) // reported >= 3

	// Credential submitter: low score, no positive badges.
	lo, lb := computeEngagement(RiskScore{Sims: 4, Clicked: 4, Submitted: 4})
	c.Assert(lo < hi, check.Equals, true)
	c.Assert(len(lb), check.Equals, 0)

	// No activity → neutral base 50, no badges.
	mid, mb := computeEngagement(RiskScore{})
	c.Assert(mid, check.Equals, 50)
	c.Assert(len(mb), check.Equals, 0)
}

// TestGetEngagementReport pins the individuals + department leaderboards.
func (s *ModelsSuite) TestGetEngagementReport(c *check.C) {
	campaign := s.createCampaign(c) // recipients test1..4@example.com
	c.Assert(len(campaign.Results), check.Equals, 4)

	place := func(email, dept string) {
		_, err := UpsertRecipient(db, 1, BaseRecipient{Email: email, Department: dept})
		c.Assert(err, check.Equals, nil)
	}
	place("test1@example.com", "Eksploatacja")
	place("test2@example.com", "Eksploatacja")
	place("test3@example.com", "IT")
	place("test4@example.com", "IT")
	// test1 reported (engaged); test3 submitted credentials (low).
	c.Assert(db.Model(&Result{}).Where("campaign_id=? AND email=?", campaign.Id, "test1@example.com").
		Update("reported", true).Error, check.Equals, nil)
	c.Assert(db.Model(&Result{}).Where("campaign_id=? AND email=?", campaign.Id, "test3@example.com").
		Update("status", EventDataSubmit).Error, check.Equals, nil)

	rep, err := GetEngagementReport(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(rep.Individuals), check.Equals, 4)
	// Individuals sorted by score desc.
	for i := 1; i < len(rep.Individuals); i++ {
		c.Assert(rep.Individuals[i-1].Score >= rep.Individuals[i].Score, check.Equals, true)
	}
	// The reporter ranks above the submitter.
	var reporterScore, submitterScore int
	for _, e := range rep.Individuals {
		if e.Email == "test1@example.com" {
			reporterScore = e.Score
		}
		if e.Email == "test3@example.com" {
			submitterScore = e.Score
		}
	}
	c.Assert(reporterScore > submitterScore, check.Equals, true)

	// Two departments, each with 2 members; Eksploatacja (has the reporter)
	// outranks IT (has the submitter).
	c.Assert(len(rep.Departments), check.Equals, 2)
	c.Assert(rep.Departments[0].Department, check.Equals, "Eksploatacja")
	c.Assert(rep.Departments[0].Members, check.Equals, int64(2))
	c.Assert(rep.Departments[0].AvgScore >= rep.Departments[1].AvgScore, check.Equals, true)
}
