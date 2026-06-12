-- SQL in section 'Up' is executed when this migration is applied
-- Phase 9: teachable moments. Mirrors the sqlite migration. See the
-- sqlite copy for the design notes.
ALTER TABLE campaigns ADD COLUMN teachable_moment boolean NOT NULL DEFAULT 0;
