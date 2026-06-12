package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) makeModule(ch *check.C) TrainingModule {
	m := TrainingModule{Name: "Mod", ContentType: TrainingContentHTML, HTML: "<p>hi</p>", UserID: 1}
	ch.Assert(PostTrainingModule(&m), check.Equals, nil)
	return m
}

func (s *ModelsSuite) TestCreateEnrollmentByEmail(ch *check.C) {
	m := s.makeModule(ch)
	br := BaseRecipient{Email: "learner@example.com", FirstName: "Lee"}

	e, err := CreateEnrollmentByEmail(1, br, m.Id)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(e.Id > 0, check.Equals, true)
	ch.Assert(e.Status, check.Equals, EnrollmentAssigned)
	ch.Assert(len(e.Token), check.Equals, 64) // 32 random bytes, hex
	ch.Assert(e.RecipientID > 0, check.Equals, true)

	// The recipient was upserted by 10a's UpsertRecipient.
	rec, err := GetRecipientByEmail("learner@example.com", 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(rec.Id, check.Equals, e.RecipientID)

	// Lookup by token works and is the portal's entry point.
	byTok, err := GetEnrollmentByToken(e.Token)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(byTok.Id, check.Equals, e.Id)
}

func (s *ModelsSuite) TestCreateEnrollmentRejectsBadInput(ch *check.C) {
	m := s.makeModule(ch)
	// Empty email.
	_, err := CreateEnrollmentByEmail(1, BaseRecipient{}, m.Id)
	ch.Assert(err, check.Equals, ErrEnrollmentRecipientRequired)
	// Unknown module.
	_, err = CreateEnrollmentByEmail(1, BaseRecipient{Email: "a@b.com"}, 99999)
	ch.Assert(err, check.Equals, ErrEnrollmentModuleNotFound)
	// Module owned by a different operator must not be enrollable by uid 1.
	other := TrainingModule{Name: "Other", ContentType: TrainingContentHTML, HTML: "<p>x</p>", UserID: 2}
	ch.Assert(PostTrainingModule(&other), check.Equals, nil)
	_, err = CreateEnrollmentByEmail(1, BaseRecipient{Email: "a@b.com"}, other.Id)
	ch.Assert(err, check.Equals, ErrEnrollmentModuleNotFound)
}

func (s *ModelsSuite) TestEnrollmentStatusTransitions(ch *check.C) {
	m := s.makeModule(ch)
	e, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "x@y.com"}, m.Id)
	ch.Assert(err, check.Equals, nil)

	ch.Assert(e.MarkStarted(), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentStarted)
	ch.Assert(e.StartedDate.IsZero(), check.Equals, false)

	ch.Assert(e.MarkCompleted(), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentCompleted)
	ch.Assert(e.CompletedDate.IsZero(), check.Equals, false)
}

// TestMarkStartedNeverDowngradesCompleted is the discriminating test for the
// revisit bug: a learner who reopens the portal after completing must not have
// their enrollment flipped completed -> started.
func (s *ModelsSuite) TestMarkStartedNeverDowngradesCompleted(ch *check.C) {
	m := s.makeModule(ch)
	e, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "z@y.com"}, m.Id)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(e.MarkCompleted(), check.Equals, nil)
	completedAt := e.CompletedDate

	// Simulate a portal revisit.
	ch.Assert(e.MarkStarted(), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentCompleted)

	// And it stayed completed in the database.
	reloaded, err := GetEnrollmentByToken(e.Token)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(reloaded.Status, check.Equals, EnrollmentCompleted)
	ch.Assert(reloaded.CompletedDate.Equal(completedAt), check.Equals, true)
}
