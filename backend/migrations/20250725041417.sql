-- Modify "repo_caches" table
ALTER TABLE "public"."repo_caches" ADD COLUMN "terragrunt_atlantis_config" bytea NULL;
