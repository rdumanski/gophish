package models

import "gopkg.in/check.v1"

// TestGetCampaignSummaries is a regression test for the GORM v2 schema-parse
// failure on CampaignSummary.Stats (it must be gorm:"-"). Before the fix this
// returned a 500-causing error whenever any campaign existed, hanging the
// dashboard and campaigns list.
func (s *ModelsSuite) TestGetCampaignSummaries(ch *check.C) {
	s.createCampaign(ch)

	sum, err := GetCampaignSummaries(1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(sum.Total, check.Equals, int64(1))
	ch.Assert(len(sum.Campaigns), check.Equals, 1)
	// Stats is computed and attached after the query.
	ch.Assert(sum.Campaigns[0].Stats.Total > 0, check.Equals, true)
}
