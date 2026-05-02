-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `imap` ADD COLUMN ignore_cert_errors BOOLEAN;
