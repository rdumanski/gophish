-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS sms_logs;
DROP TABLE IF EXISTS sms_profiles;
ALTER TABLE templates DROP COLUMN channel;
ALTER TABLE campaigns DROP COLUMN sms_profile_id;
ALTER TABLE campaigns DROP COLUMN channel;
ALTER TABLE email_requests DROP COLUMN phone;
ALTER TABLE results DROP COLUMN phone;
ALTER TABLE targets DROP COLUMN phone;
