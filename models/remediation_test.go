package models

import (
	check "gopkg.in/check.v1"
)

func remEnrollCount(rid, moduleID int64) int64 {
	var n int64
	db.Model(&Enrollment{}).Where("recipient_id=? AND module_id=?", rid, moduleID).Count(&n)
	return n
}

func (s *ModelsSuite) TestRemediationSettingsRoundTrip(c *check.C) {
	mod := s.makeModule(c)
	c.Assert(PutRemediationSettings(&RemediationSettings{
		UserID: 1, Enabled: true, ModuleID: mod.Id, TriggerOn: RemediationOnSubmit,
		PortalBaseURL: "https://x.example",
	}), check.Equals, nil)

	got := GetRemediationSettings(1)
	c.Assert(got.Enabled, check.Equals, true)
	c.Assert(got.ModuleID, check.Equals, mod.Id)
	c.Assert(got.TriggerOn, check.Equals, RemediationOnSubmit)

	// Upsert (one row per user): change + put again, no duplicate.
	got.PortalBaseURL = "https://y.example"
	c.Assert(PutRemediationSettings(&got), check.Equals, nil)
	c.Assert(GetRemediationSettings(1).PortalBaseURL, check.Equals, "https://y.example")
	var rows int64
	db.Model(&RemediationSettings{}).Where("user_id=?", 1).Count(&rows)
	c.Assert(rows, check.Equals, int64(1))
}

// TestTriggerRemediation pins the failure → auto-enroll loop: threshold respect,
// click-or-submit vs submit-only, idempotency, and the disabled no-op. SMTP is
// left unset so the email step is skipped (enrollment logic tested standalone).
func (s *ModelsSuite) TestTriggerRemediation(c *check.C) {
	mod := s.makeModule(c)
	rid, err := UpsertRecipient(db, 1, BaseRecipient{Email: "fail@x.com", FirstName: "F"})
	c.Assert(err, check.Equals, nil)
	clicked := Result{UserID: 1, RecipientID: rid, Status: EventClicked,
		BaseRecipient: BaseRecipient{Email: "fail@x.com", FirstName: "F"}}

	// Disabled → no enrollment.
	c.Assert(TriggerRemediation(clicked), check.Equals, nil)
	c.Assert(remEnrollCount(rid, mod.Id), check.Equals, int64(0))

	// Submit-only → a click must NOT enroll.
	c.Assert(PutRemediationSettings(&RemediationSettings{UserID: 1, Enabled: true, ModuleID: mod.Id, TriggerOn: RemediationOnSubmit}), check.Equals, nil)
	c.Assert(TriggerRemediation(clicked), check.Equals, nil)
	c.Assert(remEnrollCount(rid, mod.Id), check.Equals, int64(0))

	// Click-or-submit → a click enrolls.
	c.Assert(PutRemediationSettings(&RemediationSettings{UserID: 1, Enabled: true, ModuleID: mod.Id, TriggerOn: RemediationOnClickOrSubmit}), check.Equals, nil)
	c.Assert(TriggerRemediation(clicked), check.Equals, nil)
	c.Assert(remEnrollCount(rid, mod.Id), check.Equals, int64(1))

	// Idempotent: a second failure doesn't double-enroll.
	c.Assert(TriggerRemediation(clicked), check.Equals, nil)
	c.Assert(remEnrollCount(rid, mod.Id), check.Equals, int64(1))

	// No recipient → no-op.
	c.Assert(TriggerRemediation(Result{UserID: 1, Status: EventDataSubmit}), check.Equals, nil)
}
