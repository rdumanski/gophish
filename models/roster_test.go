package models

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/check.v1"
)

func csvOf(emails ...string) []byte {
	var b strings.Builder
	b.WriteString("First Name,Email\n")
	for i, e := range emails {
		b.WriteString(fmt.Sprintf("User%d,%s\n", i, e))
	}
	return []byte(b.String())
}

func rosterMsg(seq uint32, date time.Time, from, subject string, csv []byte) RosterMessage {
	return RosterMessage{
		SeqNum:      seq,
		Date:        date,
		From:        from,
		Subject:     subject,
		Attachments: []RosterAttachment{{Filename: "roster.csv", Content: csv}},
	}
}

func (s *ModelsSuite) groupEmails(ch *check.C, name string) map[string]bool {
	g, err := GetGroupByName(name, 1)
	ch.Assert(err, check.Equals, nil)
	return emailSet(g.Targets)
}

func (s *ModelsSuite) TestProcessRosterCreatesGroup(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster"}
	res := ProcessRoster(&rs, []RosterMessage{
		rosterMsg(1, time.Now(), "hr@corp.com", "weekly roster", csvOf("a@x.com", "b@x.com")),
	})
	ch.Assert(res.Applied, check.Equals, true)
	ch.Assert(res.Total, check.Equals, 2)
	ch.Assert(res.Added, check.Equals, 2)
	ch.Assert(res.Removed, check.Equals, 0)
	emails := s.groupEmails(ch, "Roster")
	ch.Assert(len(emails), check.Equals, 2)
	ch.Assert(emails["a@x.com"], check.Equals, true)
}

func (s *ModelsSuite) TestProcessRosterUpdatesMembership(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster", MaxRemovalPercent: 50}
	ProcessRoster(&rs, []RosterMessage{rosterMsg(1, time.Now(), "hr@corp.com", "r", csvOf("a@x.com", "b@x.com", "c@x.com"))})

	// Drop c, add d (remove 1 of 3 = 33%, under the 50% cap).
	res := ProcessRoster(&rs, []RosterMessage{rosterMsg(2, time.Now(), "hr@corp.com", "r", csvOf("a@x.com", "b@x.com", "d@x.com"))})
	ch.Assert(res.Applied, check.Equals, true)
	ch.Assert(res.Added, check.Equals, 1)
	ch.Assert(res.Removed, check.Equals, 1)
	emails := s.groupEmails(ch, "Roster")
	ch.Assert(emails["d@x.com"], check.Equals, true)
	ch.Assert(emails["c@x.com"], check.Equals, false)
}

func (s *ModelsSuite) TestProcessRosterDeltaGuardRefuses(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster"} // default 30% cap
	ProcessRoster(&rs, []RosterMessage{rosterMsg(1, time.Now(), "hr@corp.com", "r", csvOf("a@x.com", "b@x.com", "c@x.com", "d@x.com"))})

	// A truncated CSV with just 1 of 4 would remove 75% — must be refused.
	res := ProcessRoster(&rs, []RosterMessage{rosterMsg(2, time.Now(), "hr@corp.com", "r", csvOf("a@x.com"))})
	ch.Assert(res.Applied, check.Equals, false)
	ch.Assert(strings.Contains(res.Message, "refused"), check.Equals, true)
	// Group is unchanged — still 4 members.
	ch.Assert(len(s.groupEmails(ch, "Roster")), check.Equals, 4)
}

func (s *ModelsSuite) TestProcessRosterLatestWins(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster"}
	older := rosterMsg(1, time.Now().Add(-time.Hour), "hr@corp.com", "r", csvOf("a@x.com", "b@x.com"))
	newer := rosterMsg(2, time.Now(), "hr@corp.com", "r", csvOf("a@x.com", "c@x.com"))
	// Pass newest first to prove it sorts by date, not slice order.
	res := ProcessRoster(&rs, []RosterMessage{newer, older})
	ch.Assert(res.Applied, check.Equals, true)
	emails := s.groupEmails(ch, "Roster")
	ch.Assert(emails["c@x.com"], check.Equals, true)
	ch.Assert(emails["b@x.com"], check.Equals, false)
	// Both messages were consumed (marked read).
	ch.Assert(len(res.ConsumedSeqNums), check.Equals, 2)
}

func (s *ModelsSuite) TestProcessRosterSubjectToken(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster", SubjectToken: "SECRET123"}
	// Wrong/absent token -> ignored.
	res := ProcessRoster(&rs, []RosterMessage{rosterMsg(1, time.Now(), "hr@corp.com", "weekly roster", csvOf("a@x.com"))})
	ch.Assert(res.Applied, check.Equals, false)
	ch.Assert(strings.Contains(res.Message, "no matching"), check.Equals, true)
	// Correct token -> accepted.
	res = ProcessRoster(&rs, []RosterMessage{rosterMsg(2, time.Now(), "hr@corp.com", "weekly roster SECRET123", csvOf("a@x.com"))})
	ch.Assert(res.Applied, check.Equals, true)
}

func (s *ModelsSuite) TestProcessRosterFromFilter(ch *check.C) {
	rs := RosterSource{UserID: 1, TargetGroup: "Roster", FromFilter: "hr@corp.com"}
	res := ProcessRoster(&rs, []RosterMessage{rosterMsg(1, time.Now(), "attacker@evil.com", "r", csvOf("a@x.com"))})
	ch.Assert(res.Applied, check.Equals, false)
	res = ProcessRoster(&rs, []RosterMessage{rosterMsg(2, time.Now(), "HR@corp.com", "r", csvOf("a@x.com"))})
	ch.Assert(res.Applied, check.Equals, true)
}
