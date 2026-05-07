-- SQL in section 'Up' is executed when this migration is applied
-- Phase 7a.1: track AI-generated templates so audit logs can distinguish
-- hand-written from LLM-drafted content. Default NULL for existing rows
-- and any template the admin authors directly.
ALTER TABLE templates ADD COLUMN generated_by varchar(255);
