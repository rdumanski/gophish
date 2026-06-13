package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) validQuiz(moduleID int64) Quiz {
	return Quiz{
		ModuleID:      moduleID,
		Title:         "Phishing Basics",
		PassThreshold: 70,
		Questions: []QuizQuestion{
			{Prompt: "What is a red flag?", Options: []QuizOption{
				{Text: "Urgency", IsCorrect: true},
				{Text: "A normal greeting", IsCorrect: false},
			}},
			{Prompt: "What should you do?", Options: []QuizOption{
				{Text: "Click fast", IsCorrect: false},
				{Text: "Report it", IsCorrect: true},
			}},
		},
	}
}

func (s *ModelsSuite) TestPostAndGetQuiz(ch *check.C) {
	m := s.makeModule(ch)
	q := s.validQuiz(m.Id)
	ch.Assert(PostQuiz(&q, 1), check.Equals, nil)
	ch.Assert(q.Id > 0, check.Equals, true)

	got, err := GetQuiz(q.Id, 1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(got.Title, check.Equals, "Phishing Basics")
	ch.Assert(len(got.Questions), check.Equals, 2)
	// Ordering preserved and options loaded.
	ch.Assert(got.Questions[0].Prompt, check.Equals, "What is a red flag?")
	ch.Assert(len(got.Questions[0].Options), check.Equals, 2)
	ch.Assert(got.Questions[0].Options[0].IsCorrect, check.Equals, true)

	// List view carries a question count.
	list, err := GetQuizzes(1)
	ch.Assert(err, check.Equals, nil)
	ch.Assert(len(list), check.Equals, 1)
	ch.Assert(list[0].QuestionCount, check.Equals, int64(2))
}

func (s *ModelsSuite) TestQuizValidation(ch *check.C) {
	m := s.makeModule(ch)
	good := s.validQuiz(m.Id)

	noTitle := good
	noTitle.Title = ""
	ch.Assert(noTitle.Validate(), check.Equals, ErrQuizTitleNotSpecified)

	badThresh := good
	badThresh.PassThreshold = 150
	ch.Assert(badThresh.Validate(), check.Equals, ErrQuizBadThreshold)

	noQ := good
	noQ.Questions = nil
	ch.Assert(noQ.Validate(), check.Equals, ErrQuizNoQuestions)

	oneOption := Quiz{Title: "t", Questions: []QuizQuestion{{Prompt: "p", Options: []QuizOption{{Text: "only", IsCorrect: true}}}}}
	ch.Assert(oneOption.Validate(), check.Equals, ErrQuizQuestionOptions)

	noCorrect := Quiz{Title: "t", Questions: []QuizQuestion{{Prompt: "p", Options: []QuizOption{{Text: "a"}, {Text: "b"}}}}}
	ch.Assert(noCorrect.Validate(), check.Equals, ErrQuizQuestionNoCorrect)

	// Unknown module rejected at Post.
	bad := s.validQuiz(99999)
	ch.Assert(PostQuiz(&bad, 1), check.Equals, ErrEnrollmentModuleNotFound)
}

func (s *ModelsSuite) TestPutQuizReplacesChildren(ch *check.C) {
	m := s.makeModule(ch)
	q := s.validQuiz(m.Id)
	ch.Assert(PostQuiz(&q, 1), check.Equals, nil)

	// Replace with a single new question.
	q.Title = "Updated"
	q.Questions = []QuizQuestion{
		{Prompt: "New?", Options: []QuizOption{{Text: "Yes", IsCorrect: true}, {Text: "No"}}},
	}
	ch.Assert(PutQuiz(&q, 1), check.Equals, nil)

	got, _ := GetQuiz(q.Id, 1)
	ch.Assert(got.Title, check.Equals, "Updated")
	ch.Assert(len(got.Questions), check.Equals, 1)
	ch.Assert(got.Questions[0].Prompt, check.Equals, "New?")

	// No orphaned options remain.
	var optCount int64
	db.Model(&QuizOption{}).Count(&optCount)
	ch.Assert(optCount, check.Equals, int64(2))
}

func (s *ModelsSuite) TestScoreQuiz(ch *check.C) {
	m := s.makeModule(ch)
	q := s.validQuiz(m.Id)
	q.PassThreshold = 60
	ch.Assert(PostQuiz(&q, 1), check.Equals, nil)
	got, _ := GetQuiz(q.Id, 1)

	correctOf := func(qq QuizQuestion) int64 {
		for _, o := range qq.Options {
			if o.IsCorrect {
				return o.Id
			}
		}
		return 0
	}
	wrongOf := func(qq QuizQuestion) int64 {
		for _, o := range qq.Options {
			if !o.IsCorrect {
				return o.Id
			}
		}
		return 0
	}

	// All correct -> 100%, pass.
	all := map[int64]int64{
		got.Questions[0].Id: correctOf(got.Questions[0]),
		got.Questions[1].Id: correctOf(got.Questions[1]),
	}
	score, passed := ScoreQuiz(got, all)
	ch.Assert(score, check.Equals, 100)
	ch.Assert(passed, check.Equals, true)

	// One of two correct -> 50%, below 60% threshold, fail.
	half := map[int64]int64{
		got.Questions[0].Id: correctOf(got.Questions[0]),
		got.Questions[1].Id: wrongOf(got.Questions[1]),
	}
	score, passed = ScoreQuiz(got, half)
	ch.Assert(score, check.Equals, 50)
	ch.Assert(passed, check.Equals, false)

	// No answers -> 0%, fail.
	score, passed = ScoreQuiz(got, map[int64]int64{})
	ch.Assert(score, check.Equals, 0)
	ch.Assert(passed, check.Equals, false)
}

func (s *ModelsSuite) TestRecordQuizResultGatesCompletion(ch *check.C) {
	m := s.makeModule(ch)
	e, err := CreateEnrollmentByEmail(1, BaseRecipient{Email: "q@y.com"}, m.Id)
	ch.Assert(err, check.Equals, nil)

	// Failing records the score but does not complete.
	ch.Assert(e.RecordQuizResult(40, false), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentStarted)
	ch.Assert(e.QuizScore, check.Equals, 40)

	// Passing completes.
	ch.Assert(e.RecordQuizResult(90, true), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentCompleted)
	ch.Assert(e.QuizScore, check.Equals, 90)

	// A later failing attempt must NOT downgrade a completed enrollment.
	ch.Assert(e.RecordQuizResult(10, false), check.Equals, nil)
	ch.Assert(e.Status, check.Equals, EnrollmentCompleted)
}

func (s *ModelsSuite) TestDeleteQuizRemovesChildren(ch *check.C) {
	m := s.makeModule(ch)
	q := s.validQuiz(m.Id)
	ch.Assert(PostQuiz(&q, 1), check.Equals, nil)

	ch.Assert(DeleteQuiz(q.Id, 1), check.Equals, nil)
	var qCount, optCount int64
	db.Model(&QuizQuestion{}).Count(&qCount)
	db.Model(&QuizOption{}).Count(&optCount)
	ch.Assert(qCount, check.Equals, int64(0))
	ch.Assert(optCount, check.Equals, int64(0))
}
