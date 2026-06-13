-- SQL in section 'Up' is executed when this migration is applied
-- Phase 12a: quizzes/assessments. A quiz belongs to a training module and has
-- ordered single-correct multiple-choice questions, each with ordered options.
-- pass_threshold is the percentage of correct answers required to pass (used by
-- the learner-portal quiz-taking flow in Phase 12b).
CREATE TABLE IF NOT EXISTS "quizzes" (
    "id" integer primary key autoincrement,
    "user_id" integer,
    "module_id" integer,
    "title" varchar(255),
    "pass_threshold" integer NOT NULL DEFAULT 70,
    "modified_date" datetime
);

CREATE TABLE IF NOT EXISTS "quiz_questions" (
    "id" integer primary key autoincrement,
    "quiz_id" integer,
    "prompt" text,
    "order_index" integer NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "quiz_options" (
    "id" integer primary key autoincrement,
    "question_id" integer,
    "text" text,
    "is_correct" boolean NOT NULL DEFAULT 0,
    "order_index" integer NOT NULL DEFAULT 0
);
