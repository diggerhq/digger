-- Create "digger_runs" table
CREATE TABLE "public"."digger_runs" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "triggertype" text NULL,
  "pr_number" bigint NULL,
  "status" text NULL,
  "commit_id" text NULL,
  "digger_config" text NULL,
  "github_installation_id" bigint NULL,
  "repo_id" bigint NULL,
  "project_id" bigint NULL,
  "run_type" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_runs_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_digger_runs_repo" FOREIGN KEY ("repo_id") REFERENCES "public"."repos" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_runs_deleted_at" to table: "digger_runs"
CREATE INDEX "idx_digger_runs_deleted_at" ON "public"."digger_runs" ("deleted_at");
-- Create "digger_run_stages" table
CREATE TABLE "public"."digger_run_stages" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "digger_run_stage_id" text NULL,
  "project_name" text NULL,
  "status" smallint NULL,
  "run_id" bigint NULL,
  "digger_job_summary_id" bigint NULL,
  "serialized_job_spec" bytea NULL,
  "workflow_file" text NULL,
  "workflow_run_url" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_run_stages_digger_job_summary" FOREIGN KEY ("digger_job_summary_id") REFERENCES "public"."digger_job_summaries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_digger_run_stages_run" FOREIGN KEY ("run_id") REFERENCES "public"."digger_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_run_stage_id" to table: "digger_run_stages"
CREATE INDEX "idx_digger_run_stage_id" ON "public"."digger_run_stages" ("run_id");
-- Create index "idx_digger_run_stages_deleted_at" to table: "digger_run_stages"
CREATE INDEX "idx_digger_run_stages_deleted_at" ON "public"."digger_run_stages" ("deleted_at");
