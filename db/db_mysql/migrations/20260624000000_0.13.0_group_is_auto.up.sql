-- SQL in section 'Up' is executed when this migration is applied.
-- Marks system-managed org-unit groups (18b.2).
ALTER TABLE groups ADD COLUMN is_auto boolean NOT NULL DEFAULT false;
