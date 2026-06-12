-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10c: training enrollments + learner portal. Mirrors the sqlite
-- migration; see the sqlite copy for the design notes.
CREATE TABLE IF NOT EXISTS `enrollments` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `recipient_id` integer,
    `module_id` integer,
    `token` varchar(128) NOT NULL DEFAULT '',
    `status` varchar(16) NOT NULL DEFAULT 'assigned',
    `assigned_date` datetime,
    `started_date` datetime,
    `completed_date` datetime
);
CREATE UNIQUE INDEX `idx_enrollments_token` ON `enrollments` (`token`);
