-- SQL in section 'Up' is executed when this migration is applied.
-- PSE org structure (Department > Sub-Department > Wydzial) + ordered position
-- level. Mirrors the sqlite migration; see it for design notes.
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

-- email_requests embeds BaseRecipient too; mirror the columns.
ALTER TABLE email_requests ADD COLUMN department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN sub_department varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN wydzial varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN position_level varchar(255) NOT NULL DEFAULT '';
