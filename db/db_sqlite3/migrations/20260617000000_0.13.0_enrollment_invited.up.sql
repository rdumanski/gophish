-- SQL in section 'Up' is executed when this migration is applied
-- Phase 11b: enrollment invitation emails. invited_date records when an
-- enrollee was emailed their learner-portal link (NULL = not yet invited).
ALTER TABLE enrollments ADD COLUMN invited_date datetime;
