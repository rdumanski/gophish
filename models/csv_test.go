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

// TestParseCSVReaderOrgStructure covers the PSE org/position columns, including
// the disambiguation that sub-department must not be swallowed by department and
// position-level must not be swallowed by position.
func (s *ModelsSuite) TestParseCSVReaderOrgStructure(ch *check.C) {
	csv := "Email,First Name,Department,Sub-Department,Wydzial,Position,Position Level\n" +
		"a@example.com,Ann,Eksploatacja,Ruch,Wydzial Ruchu,Specjalista ds. ruchu,Specjalista\n"
	ts, err := ParseCSVReader(strings.NewReader(csv))
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(ts), check.Equals, 1)
	ch.Assert(ts[0].Department, check.Equals, "Eksploatacja")
	ch.Assert(ts[0].SubDepartment, check.Equals, "Ruch")
	ch.Assert(ts[0].Wydzial, check.Equals, "Wydzial Ruchu")
	ch.Assert(ts[0].Position, check.Equals, "Specjalista ds. ruchu")
	ch.Assert(ts[0].PositionLevel, check.Equals, "Specjalista")
}

// TestParseCSVReaderPolishHeaders covers Polish column headers from a typical
// HR/directory export.
func (s *ModelsSuite) TestParseCSVReaderPolishHeaders(ch *check.C) {
	csv := "E-mail,Imię,Nazwisko,Departament,Pod-Departament,Wydział,Stanowisko,Poziom\n" +
		"k@pse.pl,Jan,Kowalski,Departament Eksploatacji,Biuro Ruchu,Wydział Nadzoru,Kierownik Wydziału,Kierownik\n"
	ts, err := ParseCSVReader(strings.NewReader(csv))
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(ts), check.Equals, 1)
	ch.Assert(ts[0].Email, check.Equals, "k@pse.pl")
	ch.Assert(ts[0].FirstName, check.Equals, "Jan")
	ch.Assert(ts[0].LastName, check.Equals, "Kowalski")
	ch.Assert(ts[0].Department, check.Equals, "Departament Eksploatacji")
	ch.Assert(ts[0].SubDepartment, check.Equals, "Biuro Ruchu")
	ch.Assert(ts[0].Wydzial, check.Equals, "Wydział Nadzoru")
	ch.Assert(ts[0].Position, check.Equals, "Kierownik Wydziału")
	ch.Assert(ts[0].PositionLevel, check.Equals, "Kierownik")
}
