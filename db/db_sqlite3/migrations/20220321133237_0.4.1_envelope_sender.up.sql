-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE templates ADD COLUMN envelope_sender varchar(255);
