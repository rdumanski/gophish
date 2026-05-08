-- SQL in section 'Up' is executed when this migration is applied
-- Phase 7c.2: store the org-wide sandbox filter policy in DB so admins
-- can edit it from the Settings UI. Single row by convention (id=1).
CREATE TABLE phish_filter (
    id INTEGER PRIMARY KEY,
    min_click_seconds INTEGER NOT NULL DEFAULT 0,
    sandbox_ips TEXT NOT NULL DEFAULT '',
    updated_at DATETIME
);
