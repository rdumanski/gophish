package models

import (
	"fmt"
	"time"

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

// mkResult inserts a Result for a recipient at a given send-date offset (days
// from now) with the chosen reported flag — for streak tests.
func (s *ModelsSuite) mkResult(c *check.C, rid int64, days int, reported bool) {
	r := Result{
		CampaignID: 1, UserID: 1, RecipientID: rid, Status: EventSent,
		Reported: reported, SendDate: time.Now().UTC().AddDate(0, 0, days),
		BaseRecipient: BaseRecipient{Email: fmt.Sprintf("r%d-%d@x.com", rid, days)},
	}
	c.Assert(r.GenerateId(db), check.Equals, nil)
	c.Assert(db.Create(&r).Error, check.Equals, nil)
}

// TestReportStreak: leading run of most-recent reported sims; broken by a
// non-reported most-recent; future-dated sims excluded.
func (s *ModelsSuite) TestReportStreak(c *check.C) {
	rid, err := UpsertRecipient(db, 1, BaseRecipient{Email: "streaker@x.com"})
	c.Assert(err, check.Equals, nil)
	// Oldest -> newest: reported, true, true (2-day & 1-day ago), 3-day ago false.
	s.mkResult(c, rid, -3, false)
	s.mkResult(c, rid, -2, true)
	s.mkResult(c, rid, -1, true)
	// A future, unsent sim must not reset the streak.
	s.mkResult(c, rid, 5, false)
	c.Assert(reportStreak(rid), check.Equals, 2)

	// A non-reported most-recent (delivered) sim breaks it.
	s.mkResult(c, rid, 0, false)
	c.Assert(reportStreak(rid), check.Equals, 0)
}

// TestGetRecipientEngagement: single-recipient score + badges + streak.
func (s *ModelsSuite) TestGetRecipientEngagement(c *check.C) {
	rid, err := UpsertRecipient(db, 1, BaseRecipient{Email: "solo@x.com", FirstName: "Solo"})
	c.Assert(err, check.Equals, nil)
	s.mkResult(c, rid, -2, true)
	s.mkResult(c, rid, -1, true)

	eng, err := GetRecipientEngagement(rid)
	c.Assert(err, check.Equals, nil)
	c.Assert(eng.RecipientID, check.Equals, rid)
	c.Assert(eng.Score > 50, check.Equals, true) // reporting lifts above base
	c.Assert(eng.Streak, check.Equals, 2)
	c.Assert(hasBadge(eng.Badges, "reporter"), check.Equals, true)
	c.Assert(hasBadge(eng.Badges, "sharp_eye"), check.Equals, true) // never clicked
}
