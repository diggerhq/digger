-- Modify "digger_jobs" table
ALTER TABLE "public"."digger_jobs" DROP COLUMN "serialized_job", ADD COLUMN "serialized_job_spec" bytea NULL;
