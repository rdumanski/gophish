-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10a: canonical per-person "recipient" record. Mirrors the sqlite
-- migration; see the sqlite copy for the design notes. The unique index
-- caps the email prefix at 191 chars so the composite (user_id, email)
-- key stays within InnoDB's 767-byte limit under utf8mb4.
CREATE TABLE IF NOT EXISTS `recipients` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `email` varchar(255) NOT NULL DEFAULT '',
    `first_name` varchar(255),
    `last_name` varchar(255),
    `position` varchar(255),
    `phone` varchar(255) NOT NULL DEFAULT '',
    `created_date` datetime,
    `modified_date` datetime
);
CREATE UNIQUE INDEX `idx_recipients_user_email` ON `recipients` (`user_id`, `email`(191));

ALTER TABLE results ADD COLUMN recipient_id integer;
