-- Modify "digger_batches" table
ALTER TABLE "digger_batches" ADD COLUMN "commit_sha" text NULL;
-- Drop index "idx_ddr_org_repo_pr_created_at" from table: "digger_detection_runs"
DROP INDEX "idx_ddr_org_repo_pr_created_at";
-- Drop index "idx_ddr_repo_pr" from table: "digger_detection_runs"
DROP INDEX "idx_ddr_repo_pr";
-- Modify "digger_detection_runs" table
ALTER TABLE "digger_detection_runs" ALTER COLUMN "created_at" DROP NOT NULL, ALTER COLUMN "created_at" DROP DEFAULT, ALTER COLUMN "organisation_id" DROP NOT NULL, ALTER COLUMN "repo_full_name" DROP NOT NULL, ALTER COLUMN "pr_number" TYPE bigint, ALTER COLUMN "pr_number" DROP NOT NULL, ALTER COLUMN "trigger_type" DROP NOT NULL, ALTER COLUMN "trigger_action" DROP NOT NULL, ALTER COLUMN "impacted_projects_json" DROP NOT NULL;
-- Rename an index from "idx_ddr_deleted_at" to "idx_digger_detection_runs_deleted_at"
ALTER INDEX "idx_ddr_deleted_at" RENAME TO "idx_digger_detection_runs_deleted_at";
-- Create "impacted_projects" table
CREATE TABLE "impacted_projects" (
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
CREATE INDEX "idx_impacted_projects_deleted_at" ON "impacted_projects" ("deleted_at");
