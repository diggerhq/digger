-- atlas:txmode none

-- Lookups by digger_job_id were sequential scans: the model's index tag used a
-- comma instead of a semicolon, so GORM never declared it, and the index named
-- idx_digger_job_id ended up on batch_id. Built CONCURRENTLY so writes are not
-- blocked while the 3.7 GB table is indexed.
-- Create index "idx_digger_jobs_digger_job_id" to table: "digger_jobs"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_digger_jobs_digger_job_id" ON "public"."digger_jobs" ("digger_job_id");
-- Create index "idx_job_tokens_value" to table: "job_tokens"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_job_tokens_value" ON "public"."job_tokens" ("value");
-- Create index "idx_digger_batches_repo_pr" to table: "digger_batches"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_digger_batches_repo_pr" ON "public"."digger_batches" ("repo_full_name", "pr_number");
-- Create index "idx_github_digger_job_links_digger_job_id" to table: "github_digger_job_links"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_github_digger_job_links_digger_job_id" ON "public"."github_digger_job_links" ("digger_job_id");
-- Create index "idx_digger_job_parent_links_digger_job_id" to table: "digger_job_parent_links"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_digger_job_parent_links_digger_job_id" ON "public"."digger_job_parent_links" ("digger_job_id");
-- Create index "idx_parent_digger_job_id" to table: "digger_job_parent_links"
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_parent_digger_job_id" ON "public"."digger_job_parent_links" ("parent_digger_job_id");
