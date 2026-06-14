-- SQL in section 'Up' is executed when this migration is applied.
-- PSE org structure: a Department > Sub-Department > Wydzial hierarchy plus an
-- ordered position level, carried on every person-bearing table. position
-- (free-text job title) and phone already exist; these add the structured
-- org/level fields used for reporting rollups, auto-groups, and org-unit
-- targeting. Canonical home is recipients; targets/results carry copies that
-- flow through CSV/roster import and PostCampaign.
ALTER TABLE recipients ADD COLUMN department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE recipients ADD COLUMN sub_department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE recipients ADD COLUMN wydzial varchar(255) NOT NULL DEFAULT '';
ALTER TABLE recipients ADD COLUMN position_level varchar(255) NOT NULL DEFAULT '';

ALTER TABLE targets ADD COLUMN department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE targets ADD COLUMN sub_department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE targets ADD COLUMN wydzial varchar(255) NOT NULL DEFAULT '';
ALTER TABLE targets ADD COLUMN position_level varchar(255) NOT NULL DEFAULT '';

ALTER TABLE results ADD COLUMN department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN sub_department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN wydzial varchar(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN position_level varchar(255) NOT NULL DEFAULT '';

-- email_requests embeds BaseRecipient too (like targets/results), so it needs
-- the same columns or GORM's read/write of the struct fails.
ALTER TABLE email_requests ADD COLUMN department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN sub_department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN wydzial varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN position_level varchar(255) NOT NULL DEFAULT '';
