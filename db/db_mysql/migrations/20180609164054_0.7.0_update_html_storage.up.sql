-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE `templates` MODIFY html MEDIUMTEXT;
ALTER TABLE `pages` MODIFY html MEDIUMTEXT;
