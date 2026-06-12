package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) makeGroupWithTargets(ch *check.C) Group {
	g := Group{Name: "TC Group", UserID: 1}
	g.Targets = []Target{
		{BaseRecipient: BaseRecipient{Email: "a@example.com", FirstName: "A"}},
		{BaseRecipient: BaseRecipient{Email: "b@example.com", FirstName: "B"}},
		{BaseRecipient: BaseRecipient{Email: "a@example.com", FirstName: "A"}}, // dup email
	}
	ch.Assert(PostGroup(&g), check.Equals, nil)
	return g
}

func (s *ModelsSuite) TestPostTrainingCampaignBulkEnrolls(ch *check.C) {
	module := s.makeModule(ch)
	s.makeGroupWithTargets(ch)

	tc := TrainingCampaign{Name: "Q3 Awareness", ModuleID: module.Id, Groups: []Group{{Name: "TC Group"}}}
	ch.Assert(PostTrainingCampaign(&tc, 1), check.Equals, nil)
	ch.Assert(tc.Id > 0, check.Equals, true)

	// Two unique recipients (the duplicate email is deduped).
	es, err := GetEnrollments(1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(es), check.Equals, 2)
	for _, e := range es {
		ch.Assert(e.TrainingCampaignID, check.Equals, tc.Id)
		ch.Assert(e.ModuleID, check.Equals, module.Id)
	}

	// Stats start fully assigned.
	got, err := GetTrainingCampaign(tc.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.Stats.Total, check.Equals, int64(2))
	ch.Assert(got.Stats.Assigned, check.Equals, int64(2))
	ch.Assert(got.Stats.Completed, check.Equals, int64(0))

	// Complete one enrollment; stats reflect it.
	e := es[0]
	ch.Assert(e.MarkCompleted(), check.Equals, nil)
	got, _ = GetTrainingCampaign(tc.Id, 1)
	ch.Assert(got.Stats.Completed, check.Equals, int64(1))
	ch.Assert(got.Stats.Assigned, check.Equals, int64(1))
}

func (s *ModelsSuite) TestPostTrainingCampaignValidation(ch *check.C) {
	module := s.makeModule(ch)
	s.makeGroupWithTargets(ch)

	noName := TrainingCampaign{ModuleID: module.Id, Groups: []Group{{Name: "TC Group"}}}
	ch.Assert(PostTrainingCampaign(&noName, 1), check.Equals, ErrTrainingCampaignNameNotSpecified)

	noGroups := TrainingCampaign{Name: "x", ModuleID: module.Id}
	ch.Assert(PostTrainingCampaign(&noGroups, 1), check.Equals, ErrTrainingCampaignNoGroups)

	badModule := TrainingCampaign{Name: "x", ModuleID: 99999, Groups: []Group{{Name: "TC Group"}}}
	ch.Assert(PostTrainingCampaign(&badModule, 1), check.Equals, ErrEnrollmentModuleNotFound)
}

func (s *ModelsSuite) TestDeleteTrainingCampaignRemovesEnrollments(ch *check.C) {
	module := s.makeModule(ch)
	s.makeGroupWithTargets(ch)
	tc := TrainingCampaign{Name: "C", ModuleID: module.Id, Groups: []Group{{Name: "TC Group"}}}
	ch.Assert(PostTrainingCampaign(&tc, 1), check.Equals, nil)

	ch.Assert(DeleteTrainingCampaign(tc.Id, 1), check.Equals, nil)
	es, _ := GetEnrollments(1)
	ch.Assert(len(es), check.Equals, 0)
	_, err := GetTrainingCampaign(tc.Id, 1)
	ch.Assert(err, check.NotNil)
}
