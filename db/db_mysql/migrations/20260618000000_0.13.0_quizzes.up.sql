-- SQL in section 'Up' is executed when this migration is applied
-- Phase 12a: quizzes/assessments. Mirrors the sqlite migration; see the sqlite
-- copy for the design notes.
CREATE TABLE IF NOT EXISTS `quizzes` (
    `id` integer primary key auto_increment,
    `user_id` integer,
    `module_id` integer,
    `title` varchar(255),
    `pass_threshold` integer NOT NULL DEFAULT 70,
    `modified_date` datetime
);

CREATE TABLE IF NOT EXISTS `quiz_questions` (
    `id` integer primary key auto_increment,
    `quiz_id` integer,
    `prompt` text,
    `order_index` integer NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS `quiz_options` (
    `id` integer primary key auto_increment,
    `question_id` integer,
    `text` text,
    `is_correct` boolean NOT NULL DEFAULT 0,
    `order_index` integer NOT NULL DEFAULT 0
);
