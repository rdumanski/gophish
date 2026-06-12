-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10a: a canonical per-person "recipient" record, keyed by
-- (user_id, email), that outlives any single campaign. This is the entity
-- later phases hang longitudinal state on (training enrollments, quiz
-- scores, risk over time). Results gain a nullable recipient_id linked on
-- write in PostCampaign; existing rows link lazily the next time a campaign
-- is created. Targets are intentionally NOT linked here: a target row is
-- shared across users by content (see insertTargetIntoGroup), so a single
-- recipient_id on it would be ambiguous; enrollment derives the recipient
-- per target at use time instead.
CREATE TABLE IF NOT EXISTS "recipients" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "email" varchar(255) NOT NULL DEFAULT '',
    "first_name" varchar(255),
    "last_name" varchar(255),
    "position" varchar(255),
    "phone" varchar(255) NOT NULL DEFAULT '',
    "created_date" datetime,
    "modified_date" datetime
);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_recipients_user_email" ON "recipients" ("user_id", "email");

ALTER TABLE results ADD COLUMN recipient_id integer;
