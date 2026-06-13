-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE enrollments DROP COLUMN quiz_score;
