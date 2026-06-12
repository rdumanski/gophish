-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10c: training enrollments + learner portal. An enrollment links a
-- Recipient (10a) to a TrainingModule (10b) and carries a long random token
-- that grants access to the public learner portal (/learn/{token}) — the
-- learner-side analogue of a Result. The token is the only credential, so it
-- is unguessable (32 random bytes) and unique-indexed; status tracks
-- assigned -> started -> completed.
CREATE TABLE IF NOT EXISTS "enrollments" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "recipient_id" integer,
    "module_id" integer,
    "token" varchar(128) NOT NULL DEFAULT '',
    "status" varchar(16) NOT NULL DEFAULT 'assigned',
    "assigned_date" datetime,
    "started_date" datetime,
    "completed_date" datetime
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_enrollments_token" ON "enrollments" ("token");
