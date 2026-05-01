-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN reported boolean default 0;
