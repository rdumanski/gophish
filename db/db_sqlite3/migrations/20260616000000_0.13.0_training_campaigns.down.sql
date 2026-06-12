-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE enrollments DROP COLUMN training_campaign_id;
DROP TABLE IF EXISTS "training_campaigns";
