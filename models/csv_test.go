package models

import (
	"strings"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestParseCSVReader(ch *check.C) {
	csv := "First Name,Last Name,Email,Position\nAlice,A,alice@example.com,Eng\nBob,B,bob@example.com,Sales\n"
	ts, err := ParseCSVReader(strings.NewReader(csv))
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(ts), check.Equals, 2)
	ch.Assert(ts[0].Email, check.Equals, "alice@example.com")
	ch.Assert(ts[0].FirstName, check.Equals, "Alice")
	ch.Assert(ts[1].Position, check.Equals, "Sales")
}

func (s *ModelsSuite) TestParseCSVReaderStripsBOMAndReordersColumns(ch *check.C) {
	// Leading UTF-8 BOM (built from bytes) + reordered/extra columns.
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	csv := bom + "email,Department,first name\nx@example.com,IT,Xavier\n"
	ts, err := ParseCSVReader(strings.NewReader(csv))
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(ts), check.Equals, 1)
	ch.Assert(ts[0].Email, check.Equals, "x@example.com")
	ch.Assert(ts[0].FirstName, check.Equals, "Xavier")
}

func (s *ModelsSuite) TestParseCSVReaderSkipsBadRows(ch *check.C) {
	// Row 2 has an invalid email (skipped); row 3 is valid.
	csv := "Email,First Name\nnot-an-email,Bad\nok@example.com,Fine\n"
	ts, err := ParseCSVReader(strings.NewReader(csv))
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(ts), check.Equals, 1)
	ch.Assert(ts[0].Email, check.Equals, "ok@example.com")
}
