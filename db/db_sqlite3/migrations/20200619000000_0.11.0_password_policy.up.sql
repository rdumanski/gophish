-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE users ADD COLUMN password_change_required BOOLEAN;
