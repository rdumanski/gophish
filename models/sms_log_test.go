package models

import (
	"time"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestGenerateSMSLogRoundtrip(ch *check.C) {
	// Reuse the email-side campaign helper — it sets up Group, Template,
	// Page, SMTP and runs PostCampaign. That gives us valid Result rows
	// to attach SMS log entries to (Phase 8b will cut the SMTP dep when
	// the dispatcher learns to write SMS-side rows from PostCampaign).
	c := s.createCampaign(ch)
	ch.Assert(len(c.Results) > 0, check.Equals, true)
	r := c.Results[0]

	send := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	ch.Assert(GenerateSMSLog(&c, &r, send), check.IsNil)

	var got SMSLog
	ch.Assert(db.Where("r_id = ? AND campaign_id = ?", r.RID, c.Id).First(&got).Error, check.IsNil)
	ch.Assert(got.UserID, check.Equals, c.UserID)
	ch.Assert(got.SendDate.Equal(send), check.Equals, true)
	ch.Assert(got.SendAttempt, check.Equals, 0)
	ch.Assert(got.Processing, check.Equals, false)
}
