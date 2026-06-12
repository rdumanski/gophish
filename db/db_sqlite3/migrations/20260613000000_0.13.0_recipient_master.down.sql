-- SQL section 'Down' is executed when this migration is rolled back
-- Dropping the table also drops its unique index. Older sqlite (pre-3.35)
-- cannot DROP COLUMN; the drivers we ship target 3.35+ so this is safe.
ALTER TABLE results DROP COLUMN recipient_id;
DROP TABLE IF EXISTS "recipients";
