-- SQL in section 'Up' is executed when this migration is applied
-- Phase 12b: quiz-taking. quiz_score records the learner's most recent quiz
-- result (percent) for an enrollment; completion is gated on reaching the
-- quiz's pass threshold. NULL/0 until a quiz is taken.
ALTER TABLE enrollments ADD COLUMN quiz_score integer NOT NULL DEFAULT 0;
