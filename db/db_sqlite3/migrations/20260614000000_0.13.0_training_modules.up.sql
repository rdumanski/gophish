-- SQL in section 'Up' is executed when this migration is applied
-- Phase 10b: training content library. A training_module is a reusable piece
-- of awareness content the operator authors once and later assigns to people
-- (enrollment lands in Phase 10c). content_type discriminates how the learner
-- portal renders it: 'html' (inline body), 'video' (hosted video URL), or
-- 'external' (link out to a course).
CREATE TABLE IF NOT EXISTS "training_modules" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "name" varchar(255),
    "description" text,
    "content_type" varchar(16) NOT NULL DEFAULT 'html',
    "html" text,
    "url" varchar(255),
    "modified_date" datetime
);
