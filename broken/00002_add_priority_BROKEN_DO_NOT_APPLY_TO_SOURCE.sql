-- BROKEN MIGRATION — DO NOT APPLY TO THE SOURCE INSTANCE.
--
-- This is the classic migration that passes on an empty dev database and fails
-- on a populated table: adding a NOT NULL column with no DEFAULT gives Postgres
-- no value to backfill existing rows with.
--
-- It is kept here ONLY to be run against a throwaway BRANCH to capture the real
-- failure for the article. The shipped/fixed version lives at
-- migrations/00002_add_priority.sql (with DEFAULT 0).
--
-- Expected error on a table with rows:
--   ERROR:  column "priority" of relation "notes" contains null values

ALTER TABLE notes ADD COLUMN priority INT NOT NULL;
