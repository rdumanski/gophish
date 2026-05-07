-- SQL section 'Down' is executed when this migration is rolled back
-- Older sqlite (pre-3.35) cannot DROP COLUMN; schema will retain the
-- column on rollback. Sqlite drivers we ship target 3.35+ so this is
-- safe in practice, but kept defensive for older deployments.
ALTER TABLE templates DROP COLUMN generated_by;
