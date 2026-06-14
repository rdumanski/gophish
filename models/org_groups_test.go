package models

import (
	check "gopkg.in/check.v1"
)

// findGroup returns the (auto) group with the given name, or nil.
func findGroup(gs []Group, name string) *Group {
	for i := range gs {
		if gs[i].Name == name {
			return &gs[i]
		}
	}
	return nil
}

// TestRegenerateOrgGroups pins the per-org-unit auto-group rebuild: one group
// per Department / Dept-Sub / Dept-Sub-Wydzial, nested membership, idempotent.
func (s *ModelsSuite) TestRegenerateOrgGroups(c *check.C) {
	place := func(email, dept, sub, wyd string) {
		_, err := UpsertRecipient(db, 1, BaseRecipient{
			Email: email, Department: dept, SubDepartment: sub, Wydzial: wyd})
		c.Assert(err, check.Equals, nil)
	}
	place("a@x.com", "D1", "S1", "W1")
	place("b@x.com", "D1", "S1", "W1")
	place("c@x.com", "D1", "S2", "W2")
	// A recipient with no department is skipped (no targetable unit).
	place("d@x.com", "", "", "")

	n, err := RegenerateOrgGroups(1)
	c.Assert(err, check.Equals, nil)
	// D1, D1/S1, D1/S1/W1, D1/S2, D1/S2/W2
	c.Assert(n, check.Equals, 5)

	gs, err := GetGroups(1)
	c.Assert(err, check.Equals, nil)
	dept := findGroup(gs, "D1")
	c.Assert(dept, check.NotNil)
	c.Assert(dept.IsAuto, check.Equals, true)
	c.Assert(len(dept.Targets), check.Equals, 3)
	wyd := findGroup(gs, "D1 / S1 / W1")
	c.Assert(wyd, check.NotNil)
	c.Assert(len(wyd.Targets), check.Equals, 2)

	// Idempotent: a second run rebuilds to the same 5 groups (no duplication).
	n2, err := RegenerateOrgGroups(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(n2, check.Equals, 5)
	gs, _ = GetGroups(1)
	autos := 0
	for _, g := range gs {
		if g.IsAuto {
			autos++
		}
	}
	c.Assert(autos, check.Equals, 5)
}
