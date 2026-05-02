-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `templates` MODIFY html TEXT;
ALTER TABLE `pages` MODIFY html TEXT;
