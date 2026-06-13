-- SQL in section 'Up' is executed when this migration is applied
-- Phase 14a: email-to-mailbox roster sync. A roster_source watches an IMAP
-- mailbox for a CSV attachment and upserts a target group from it on a
-- schedule, so the recipient list stays current without granting the tool
-- direct directory access. subject_token is a shared secret required in the
-- email subject (the real auth control); from_filter is a convenience filter
-- only. max_removal_percent guards against a truncated CSV pruning the group.
CREATE TABLE IF NOT EXISTS "roster_sources" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "name" varchar(255),
    "enabled" boolean NOT NULL DEFAULT 0,
    "host" varchar(255),
    "port" integer,
    "username" varchar(255),
    "password" varchar(255),
    "tls" boolean NOT NULL DEFAULT 1,
    "ignore_cert_errors" boolean NOT NULL DEFAULT 0,
    "folder" varchar(255) NOT NULL DEFAULT 'INBOX',
    "from_filter" varchar(255),
    "subject_token" varchar(255),
    "target_group" varchar(255),
    "max_removal_percent" integer NOT NULL DEFAULT 30,
    "freq" integer NOT NULL DEFAULT 3600,
    "last_sync" datetime,
    "last_status" text,
    "modified_date" datetime
);
