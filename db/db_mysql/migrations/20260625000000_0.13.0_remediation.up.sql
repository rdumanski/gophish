-- SQL in section 'Up' is executed when this migration is applied.
-- Per-operator remediation auto-enrollment config (Phase 20).
CREATE TABLE IF NOT EXISTS `remediation_settings` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `enabled` boolean NOT NULL DEFAULT 0,
    `module_id` integer NOT NULL DEFAULT 0,
    `trigger_on` varchar(32) NOT NULL DEFAULT 'click_or_submit',
    `smtp_name` varchar(255) NOT NULL DEFAULT '',
    `portal_base_url` varchar(255) NOT NULL DEFAULT '',
    `updated_at` datetime
);
CREATE UNIQUE INDEX `idx_remediation_user` ON `remediation_settings` (`user_id`);
