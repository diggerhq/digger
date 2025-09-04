-- Modify "github_app_connections" table
ALTER TABLE "public"."github_app_connections" ADD COLUMN "vcs_type" text NULL DEFAULT 'bitbucket';
