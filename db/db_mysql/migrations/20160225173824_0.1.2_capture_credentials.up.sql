-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `pages` ADD COLUMN capture_credentials BOOLEAN;
ALTER TABLE `pages` ADD COLUMN capture_passwords BOOLEAN;
