-- SQL in section 'Up' is executed when this migration is applied
-- Phase 9: teachable moments. When enabled on a campaign, a recipient who
-- clicks the link (or submits the form) is shown a first-party education
-- page instead of the landing page. Default 0 so existing campaigns keep
-- rendering their landing page exactly as before.
ALTER TABLE campaigns ADD COLUMN teachable_moment boolean NOT NULL DEFAULT 0;
