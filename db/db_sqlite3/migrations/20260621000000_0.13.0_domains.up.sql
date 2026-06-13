-- SQL in section 'Up' is executed when this migration is applied
-- Phase 15a: custom-domains registry. Tracks the look-alike domains we own and
-- their role (sending identity / landing host / both) plus last health-check
-- results (DNS+HTTPS reachability for landing; SPF/DKIM/DMARC presence for
-- sending). v1 does NOT generate DKIM keys — the mail relay owns/signs the key;
-- we only record the selector and verify the published record.
CREATE TABLE IF NOT EXISTS "domains" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "name" varchar(255),
    "role" varchar(16) NOT NULL DEFAULT 'both',
    "dkim_selector" varchar(255),
    "last_checked" datetime,
    "landing_ok" boolean NOT NULL DEFAULT 0,
    "spf_ok" boolean NOT NULL DEFAULT 0,
    "dkim_ok" boolean NOT NULL DEFAULT 0,
    "dmarc_ok" boolean NOT NULL DEFAULT 0,
    "status" text,
    "modified_date" datetime
);
