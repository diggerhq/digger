-- Modify "repos" table
ALTER TABLE "public"."repos" DROP COLUMN "github_installation_id", ADD COLUMN "github_app_installation_id" bigint NULL;
