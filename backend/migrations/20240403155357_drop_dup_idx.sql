# drop the duplicate index to fix the next migration of renaming
DROP INDEX "public"."idx_digger_job_id";
DROP INDEX "idx_digger_run_queues_deleted_at";
DROP INDEX "idx_digger_run_queue_project_id";
DROP INDEX "idx_digger_run_queue_run_id";