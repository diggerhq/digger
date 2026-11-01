-- Create index "idx_digger_batches_pr_commit" to table: "digger_batches"
CREATE INDEX "idx_digger_batches_pr_commit" ON "public"."digger_batches" ("repo_full_name", "pr_number", "commit_sha");
-- Create index "idx_parent_digger_job_id" to table: "digger_job_parent_links"
CREATE INDEX "idx_parent_digger_job_id" ON "public"."digger_job_parent_links" ("parent_digger_job_id");
