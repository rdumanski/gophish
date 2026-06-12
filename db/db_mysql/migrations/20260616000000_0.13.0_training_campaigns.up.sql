-- SQL in section 'Up' is executed when this migration is applied
-- Phase 11a: training campaigns. Mirrors the sqlite migration; see the sqlite
-- copy for the design notes.
CREATE TABLE IF NOT EXISTS `training_campaigns` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `name` varchar(255),
    `module_id` integer,
    `due_date` datetime,
    `created_date` datetime
);

ALTER TABLE enrollments ADD COLUMN training_campaign_id integer;
