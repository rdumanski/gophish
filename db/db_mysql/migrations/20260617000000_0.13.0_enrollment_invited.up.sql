-- SQL in section 'Up' is executed when this migration is applied
-- Phase 11b: enrollment invitation emails. Mirrors the sqlite migration.
ALTER TABLE enrollments ADD COLUMN invited_date datetime;
