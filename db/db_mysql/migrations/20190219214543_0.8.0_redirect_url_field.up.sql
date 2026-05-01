-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `pages` MODIFY redirect_url TEXT;
