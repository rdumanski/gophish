-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE users ADD COLUMN last_login datetime;
