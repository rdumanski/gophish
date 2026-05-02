-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `pages` MODIFY redirect_url VARCHAR(255);
