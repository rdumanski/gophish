package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestComputeRiskScore(ch *check.C) {
	// No data -> baseline 30.
	ch.Assert(computeRiskScore(RiskScore{}), check.Equals, 30)

	// Submitted credentials on the only sim: 30 + 40 = 70.
	ch.Assert(computeRiskScore(RiskScore{Sims: 1, Clicked: 1, Submitted: 1}), check.Equals, 70)

	// Clicked but didn't submit: 30 + 20 = 50.
	ch.Assert(computeRiskScore(RiskScore{Sims: 1, Clicked: 1}), check.Equals, 50)

	// Reported the sim: 30 - 15 = 15.
	ch.Assert(computeRiskScore(RiskScore{Sims: 1, Reported: 1}), check.Equals, 15)

	// Completed all assigned training: 30 - 25 = 5.
	ch.Assert(computeRiskScore(RiskScore{TrainingAssigned: 2, TrainingCompleted: 2}), check.Equals, 5)

	// Clamped to 100 at the high end.
	ch.Assert(computeRiskScore(RiskScore{Sims: 1, Submitted: 1, Clicked: 1}) <= 100, check.Equals, true)
}

func (s *ModelsSuite) TestGetRiskScoresAggregates(ch *check.C) {
	// A campaign creates results + recipients; mark one result submitted.
	c := s.createCampaign(ch)
	ch.Assert(len(c.Results) >= 2, check.Equals, true)

	r0 := c.Results[0]
	ch.Assert(r0.HandleClickedLink(EventDetails{}), check.Equals, nil)
	ch.Assert(r0.HandleFormSubmit(EventDetails{}), check.Equals, nil)

	scores, err := GetRiskScores(1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(scores) >= 2, check.Equals, true)

	// Highest-risk first; the submitter leads and scores above baseline.
	ch.Assert(scores[0].Submitted, check.Equals, int64(1))
	ch.Assert(scores[0].Score > 30, check.Equals, true)
	// Sorted descending.
	for i := 1; i < len(scores); i++ {
		ch.Assert(scores[i-1].Score >= scores[i].Score, check.Equals, true)
	}
}
