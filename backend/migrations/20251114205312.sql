-- Modify "digger_batches" table
ALTER TABLE "public"."digger_batches" ADD COLUMN "commit_sha" text NULL;
-- Drop index "idx_ddr_deleted_at" from table: "digger_detection_runs"
DROP INDEX "public"."idx_ddr_deleted_at";
-- Drop index "idx_ddr_org_repo_pr_created_at" from table: "digger_detection_runs"
DROP INDEX "public"."idx_ddr_org_repo_pr_created_at";
-- Modify "digger_detection_runs" table
ALTER TABLE "public"."digger_detection_runs" ALTER COLUMN "created_at" DROP NOT NULL, ALTER COLUMN "created_at" DROP DEFAULT;
-- Create index "idx_ddr_org_repo_pr_created_at" to table: "digger_detection_runs"
CREATE INDEX "idx_ddr_org_repo_pr_created_at" ON "public"."digger_detection_runs" ("organisation_id", "repo_full_name", "pr_number");
-- Create index "idx_digger_detection_runs_deleted_at" to table: "digger_detection_runs"
CREATE INDEX "idx_digger_detection_runs_deleted_at" ON "public"."digger_detection_runs" ("deleted_at");
-- Create "impacted_projects" table
CREATE TABLE "public"."impacted_projects" (
  "id" text NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "repo_full_name" text NULL,
  "commit_sha" text NULL,
  "project_name" text NULL,
  "planned" boolean NULL,
  "applied" boolean NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_impacted_projects_deleted_at" to table: "impacted_projects"
CREATE INDEX "idx_impacted_projects_deleted_at" ON "public"."impacted_projects" ("deleted_at");
