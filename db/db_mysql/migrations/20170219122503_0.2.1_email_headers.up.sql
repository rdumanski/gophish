-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS `headers` (
	id integer primary key auto_increment,
	`key` varchar(255),
	`value` varchar(255),
	`smtp_id` bigint
);
