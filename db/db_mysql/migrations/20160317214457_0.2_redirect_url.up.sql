-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `pages` ADD COLUMN redirect_url VARCHAR(255);
