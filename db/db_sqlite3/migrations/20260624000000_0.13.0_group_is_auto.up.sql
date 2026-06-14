-- SQL in section 'Up' is executed when this migration is applied.
-- Marks system-managed groups auto-generated per org unit (18b.2) so they can
-- be regenerated/badged distinctly from operator-created groups.
ALTER TABLE groups ADD COLUMN is_auto boolean NOT NULL DEFAULT 0;
