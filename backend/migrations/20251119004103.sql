-- Modify "digger_batches" table
ALTER TABLE "public"."digger_batches" ADD COLUMN "check_run_url" text NULL;
-- Modify "digger_jobs" table
ALTER TABLE "public"."digger_jobs" ADD COLUMN "check_run_url" text NULL;
