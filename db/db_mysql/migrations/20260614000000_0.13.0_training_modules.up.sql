-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10b: training content library. Mirrors the sqlite migration; see the
-- sqlite copy for the design notes.
CREATE TABLE IF NOT EXISTS `training_modules` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `name` varchar(255),
    `description` text,
    `content_type` varchar(16) NOT NULL DEFAULT 'html',
    `html` longtext,
    `url` varchar(255),
    `modified_date` datetime
);
