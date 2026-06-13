-- SQL in section 'Up' is executed when this migration is applied
-- Phase 12b: quiz-taking. Mirrors the sqlite migration.
ALTER TABLE enrollments ADD COLUMN quiz_score integer NOT NULL DEFAULT 0;
