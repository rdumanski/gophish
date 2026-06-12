-- SQL in section 'Up' is executed when this migration is applied
-- Phase 11a: training campaigns. A training_campaign assigns one module to one
-- or more groups with a due date; creating it bulk-creates an enrollment per
-- unique recipient. enrollments gain a nullable training_campaign_id so a
-- campaign's completion can be aggregated (NULL = a standalone enrollment
-- created directly via the 10c API).
CREATE TABLE IF NOT EXISTS "training_campaigns" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "name" varchar(255),
    "module_id" integer,
    "due_date" datetime,
    "created_date" datetime
);

ALTER TABLE enrollments ADD COLUMN training_campaign_id integer;
