-- SQL in section 'Up' is executed when this migration is applied.
-- Per-user UI language preference for bilingual (Polish/English) localization.
ALTER TABLE users ADD COLUMN language varchar(10) NOT NULL DEFAULT 'en';
