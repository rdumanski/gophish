-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `smtp` ADD COLUMN ignore_cert_errors BOOLEAN;
