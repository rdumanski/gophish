package models

import (
	check "gopkg.in/check.v1"
)

// TestPostIMAPRoundTrip pins the GORM v2 fix in PostIMAP. The IMAP model has
// no primary key, so db.Save took the UPDATE path and emitted an UPDATE with
// no WHERE — rejected by v2's global-update guard with "WHERE conditions
// required", which broke saving IMAP reporting settings in the UI. PostIMAP
// now uses db.Create (after DeleteIMAP clears any existing row). Use localhost
// so IMAP.Validate's net.LookupHost succeeds offline.
func (s *ModelsSuite) TestPostIMAPRoundTrip(c *check.C) {
	in := &IMAP{
		UserID:   1,
		Enabled:  true,
		Host:     "localhost",
		Port:     993,
		Username: "reporter",
		Password: "secret",
		TLS:      true,
	}
	err := PostIMAP(in, 1)
	c.Assert(err, check.Equals, nil)

	got, err := GetIMAP(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(got), check.Equals, 1)
	c.Assert(got[0].Host, check.Equals, "localhost")
	c.Assert(got[0].Username, check.Equals, "reporter")
	c.Assert(got[0].Enabled, check.Equals, true)
	// Folder defaults to INBOX during Validate when left blank.
	c.Assert(got[0].Folder, check.Equals, DefaultIMAPFolder)

	// A second save must replace, not duplicate or error (the delete-then-
	// create path).
	in.Host = "127.0.0.1"
	err = PostIMAP(in, 1)
	c.Assert(err, check.Equals, nil)
	got, err = GetIMAP(1)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(got), check.Equals, 1)
	c.Assert(got[0].Host, check.Equals, "127.0.0.1")
}
