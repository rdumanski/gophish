-- SQL in section 'Up' is executed when this migration is applied
-- Phase 15a: custom-domains registry. Mirrors the sqlite migration.
CREATE TABLE IF NOT EXISTS `domains` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `name` varchar(255),
    `role` varchar(16) NOT NULL DEFAULT 'both',
    `dkim_selector` varchar(255),
    `last_checked` datetime,
    `landing_ok` boolean NOT NULL DEFAULT 0,
    `spf_ok` boolean NOT NULL DEFAULT 0,
    `dkim_ok` boolean NOT NULL DEFAULT 0,
    `dmarc_ok` boolean NOT NULL DEFAULT 0,
    `status` text,
    `modified_date` datetime
);
