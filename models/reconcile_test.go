package models

import "gopkg.in/check.v1"

func (s *ModelsSuite) TestReconcileReports(ch *check.C) {
	c := s.createCampaign(ch)
	ch.Assert(len(c.Results) >= 3, check.Equals, true)

	emailA := c.Results[0].Email
	ridB := c.Results[1].RID

	// Mark one by email, one by rid, plus a bogus + a duplicate of A.
	res, err := ReconcileReports(c.Id, 1, []string{emailA, ridB, "nobody@example.com", emailA})
	ch.Assert(err, check.Equals, nil)
	ch.Assert(res.Marked, check.Equals, 2)         // A (email) + B (rid)
	ch.Assert(res.NotFound, check.Equals, 1)       // bogus
	ch.Assert(len(res.Unmatched), check.Equals, 1) // the bogus address

	// Both results are now Reported.
	r0, _ := GetResult(c.Results[0].RID)
	r1, _ := GetResult(c.Results[1].RID)
	ch.Assert(r0.Reported, check.Equals, true)
	ch.Assert(r1.Reported, check.Equals, true)

	// Re-running is idempotent: already-reported, nothing newly marked.
	res2, err := ReconcileReports(c.Id, 1, []string{emailA, ridB})
	ch.Assert(err, check.Equals, nil)
	ch.Assert(res2.Marked, check.Equals, 0)
	ch.Assert(res2.AlreadyReported, check.Equals, 2)
}

func (s *ModelsSuite) TestReconcileReportsWrongOwner(ch *check.C) {
	c := s.createCampaign(ch)
	_, err := ReconcileReports(c.Id, 2, []string{c.Results[0].Email})
	ch.Assert(err, check.NotNil) // campaign not owned by user 2
}
