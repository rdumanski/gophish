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

// TestUpsertRecipientOrgFields covers org/level fields on create, refresh, and
// the non-empty guard (empty incoming values preserve an existing placement).
func (s *ModelsSuite) TestUpsertRecipientOrgFields(c *check.C) {
	br := BaseRecipient{Email: "u@example.com", FirstName: "U",
		Department: "D1", SubDepartment: "S1", Wydzial: "W1", PositionLevel: "Specjalista"}
	id, err := UpsertRecipient(db, 1, br)
	c.Assert(err, check.Equals, nil)
	c.Assert(id > 0, check.Equals, true)

	r, err := GetRecipientByEmail("u@example.com", 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(r.Department, check.Equals, "D1")
	c.Assert(r.SubDepartment, check.Equals, "S1")
	c.Assert(r.Wydzial, check.Equals, "W1")
	c.Assert(r.PositionLevel, check.Equals, "Specjalista")

	// Refresh with new values overwrites.
	br.Department = "D2"
	_, err = UpsertRecipient(db, 1, br)
	c.Assert(err, check.Equals, nil)
	r, _ = GetRecipientByEmail("u@example.com", 1)
	c.Assert(r.Department, check.Equals, "D2")

	// Empty org values must NOT wipe the existing placement.
	_, err = UpsertRecipient(db, 1, BaseRecipient{Email: "u@example.com", FirstName: "U"})
	c.Assert(err, check.Equals, nil)
	r, _ = GetRecipientByEmail("u@example.com", 1)
	c.Assert(r.Department, check.Equals, "D2")
}

// TestPostGroupPersistsOrgFields covers the import -> target -> display path:
// org/level/phone fields survive PostGroup and come back via GetTargets.
func (s *ModelsSuite) TestPostGroupPersistsOrgFields(c *check.C) {
	g := Group{Name: "Org Group", UserID: 1}
	g.Targets = []Target{{BaseRecipient: BaseRecipient{
		Email: "o@example.com", FirstName: "O",
		Department: "Eksploatacja", SubDepartment: "Ruch", Wydzial: "Wydzial Ruchu",
		PositionLevel: "Specjalista", Phone: "+48555000111",
	}}}
	c.Assert(PostGroup(&g), check.Equals, nil)

	ts, err := GetTargets(g.Id)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(ts), check.Equals, 1)
	c.Assert(ts[0].Department, check.Equals, "Eksploatacja")
	c.Assert(ts[0].SubDepartment, check.Equals, "Ruch")
	c.Assert(ts[0].Wydzial, check.Equals, "Wydzial Ruchu")
	c.Assert(ts[0].PositionLevel, check.Equals, "Specjalista")
	c.Assert(ts[0].Phone, check.Equals, "+48555000111")
}
