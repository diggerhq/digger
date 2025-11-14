-- Modify "impacted_projects" table
ALTER TABLE "public"."impacted_projects" ADD COLUMN "pr_number" bigint NULL, ADD COLUMN "branch" text NULL;
