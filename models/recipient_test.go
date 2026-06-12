package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestUpsertRecipientCreatesThenReuses(ch *check.C) {
	br := BaseRecipient{Email: "person@example.com", FirstName: "Pat", LastName: "Example", Position: "Analyst"}

	id1, err := UpsertRecipient(db, 1, br)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(id1 > 0, check.Equals, true)

	// Same (user, email) reuses the row and refreshes the profile fields.
	br.FirstName = "Patricia"
	br.Position = "Manager"
	id2, err := UpsertRecipient(db, 1, br)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(id2, check.Equals, id1)

	r, err := GetRecipientByEmail("person@example.com", 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(r.FirstName, check.Equals, "Patricia")
	ch.Assert(r.Position, check.Equals, "Manager")
}

func (s *ModelsSuite) TestUpsertRecipientScopedByUser(ch *check.C) {
	br := BaseRecipient{Email: "shared@example.com", FirstName: "Sam"}
	id1, err := UpsertRecipient(db, 1, br)
	ch.Assert(err, check.Equals, nil)
	id2, err := UpsertRecipient(db, 2, br)
	ch.Assert(err, check.Equals, nil)
	// The same email under a different owner is a distinct person.
	ch.Assert(id1 == id2, check.Equals, false)
}

func (s *ModelsSuite) TestUpsertRecipientEmptyEmailSkips(ch *check.C) {
	id, err := UpsertRecipient(db, 1, BaseRecipient{Phone: "+15555551234"})
	ch.Assert(err, check.Equals, nil)
	ch.Assert(id, check.Equals, int64(0))

	_, err = GetRecipientByEmail("", 1)
	ch.Assert(err, check.NotNil)
}

func (s *ModelsSuite) TestPostCampaignLinksResultsToRecipients(ch *check.C) {
	campaign := s.createCampaign(ch)

	ch.Assert(len(campaign.Results) > 0, check.Equals, true)
	for _, r := range campaign.Results {
		ch.Assert(r.RecipientID > 0, check.Equals, true)
		rec, err := GetRecipientByEmail(r.Email, campaign.UserID)
		ch.Assert(err, check.Equals, nil)
		ch.Assert(rec.Id, check.Equals, r.RecipientID)
	}
}
