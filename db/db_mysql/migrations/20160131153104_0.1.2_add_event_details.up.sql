-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `events` ADD COLUMN details BLOB;
