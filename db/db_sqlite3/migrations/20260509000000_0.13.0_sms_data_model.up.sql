-- SQL in section 'Up' is executed when this migration is applied
-- Phase 8a: data-model groundwork for SMS phishing campaigns.
-- Adds an optional phone field to recipients, a channel
-- discriminator on campaigns + templates (default 'email' so
-- existing rows keep behaving exactly as before), an optional FK
-- to a new sms_profiles table, and the queue table sms_logs.
-- No worker/dispatcher/UI wiring lives here — Phase 8b adds that.

ALTER TABLE targets ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';
ALTER TABLE results ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';
-- email_requests also embeds BaseRecipient via GORM, so it needs the
-- column too — otherwise INSERTs from the test-email path fail with
-- "no such column: phone" the moment Phase 8a's struct landing rolls.
ALTER TABLE email_requests ADD COLUMN phone varchar(255) NOT NULL DEFAULT '';

ALTER TABLE campaigns ADD COLUMN channel varchar(16) NOT NULL DEFAULT 'email';
ALTER TABLE campaigns ADD COLUMN sms_profile_id integer;

ALTER TABLE templates ADD COLUMN channel varchar(16) NOT NULL DEFAULT 'email';

CREATE TABLE IF NOT EXISTS "sms_profiles" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "name" varchar(255),
    "provider" varchar(32) NOT NULL DEFAULT 'twilio',
    "account_sid" varchar(255),
    "auth_token" varchar(255),
    "from_number" varchar(32),
    "messaging_service_sid" varchar(255),
    "modified_date" datetime
);

CREATE TABLE IF NOT EXISTS "sms_logs" (
    "id" integer primary key autoincrement,
    "campaign_id" integer,
    "user_id" integer,
    "send_date" datetime,
    "send_attempt" integer,
    "r_id" varchar(255),
    "processing" boolean
);
