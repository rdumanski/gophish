-- SQL section 'Down' is executed when this migration is rolled back
-- Dropping the table also drops its unique index.
ALTER TABLE results DROP COLUMN recipient_id;
DROP TABLE IF EXISTS `recipients`;
