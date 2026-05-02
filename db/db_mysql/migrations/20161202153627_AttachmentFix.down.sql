-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE `attachments` MODIFY content TEXT;
