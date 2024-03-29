-- Modify "repos" table
ALTER TABLE "public"."repos" ADD COLUMN "repo_full_name" text NULL, ADD COLUMN "repo_organisation" text NULL, ADD COLUMN "repo_name" text NULL, ADD COLUMN "repo_url" text NULL;
