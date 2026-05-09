-- SQL in section 'Up' is executed when this migration is applied
-- Phase 8a: data-model groundwork for SMS phishing campaigns.
-- Mirrors the sqlite migration. See the sqlite copy for the
-- design notes.

ALTER TABLE targets ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';
ALTER TABLE email_requests ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';

ALTER TABLE campaigns ADD COLUMN channel varchar(16) NOT NULL DEFAULT 'email';
ALTER TABLE campaigns ADD COLUMN sms_profile_id integer;

ALTER TABLE templates ADD COLUMN channel varchar(16) NOT NULL DEFAULT 'email';

CREATE TABLE IF NOT EXISTS `sms_profiles` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `name` varchar(255),
    `provider` varchar(32) NOT NULL DEFAULT 'twilio',
    `account_sid` varchar(255),
    `auth_token` varchar(255),
    `from_number` varchar(32),
    `messaging_service_sid` varchar(255),
    `modified_date` datetime
);

CREATE TABLE IF NOT EXISTS `sms_logs` (
    `id` integer primary key auto_increment,
    `campaign_id` integer,
    `user_id` integer,
    `send_date` datetime,
    `send_attempt` integer,
    `r_id` varchar(255),
    `processing` boolean
);
