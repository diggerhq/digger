-- Modify "digger_jobs" table
ALTER TABLE "public"."digger_jobs" ADD COLUMN "reporter_type" text NULL DEFAULT 'lazy';
