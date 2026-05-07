-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE templates DROP COLUMN generated_by;
