-- SQL section 'Down' is executed when this migration is rolled back.
-- Sqlite drivers we ship target 3.35+ (DROP COLUMN supported).
ALTER TABLE recipients DROP COLUMN department;
ALTER TABLE recipients DROP COLUMN sub_department;
ALTER TABLE recipients DROP COLUMN wydzial;
ALTER TABLE recipients DROP COLUMN position_level;

ALTER TABLE targets DROP COLUMN department;
ALTER TABLE targets DROP COLUMN sub_department;
ALTER TABLE targets DROP COLUMN wydzial;
ALTER TABLE targets DROP COLUMN position_level;

ALTER TABLE results DROP COLUMN department;
ALTER TABLE results DROP COLUMN sub_department;
ALTER TABLE results DROP COLUMN wydzial;
ALTER TABLE results DROP COLUMN position_level;

ALTER TABLE email_requests DROP COLUMN department;
ALTER TABLE email_requests DROP COLUMN sub_department;
ALTER TABLE email_requests DROP COLUMN wydzial;
ALTER TABLE email_requests DROP COLUMN position_level;
