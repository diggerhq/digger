-- Modify "projects" table
ALTER TABLE "public"."projects" DROP COLUMN "repo_id", ADD COLUMN "repo_full_name" text NULL;
-- Create index "idx_project_org" to table: "projects"
DROP INDEX IF EXISTS "idx_project";
CREATE UNIQUE INDEX "idx_project_org" ON "public"."projects" ("name", "organisation_id", "repo_full_name");
