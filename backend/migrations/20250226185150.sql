-- Modify "github_app_connections" table
ALTER TABLE "public"."github_app_connections" ADD COLUMN "bitbucket_access_token_encrypted" text NULL, ADD COLUMN "bitbucket_webhook_secret_encrypted" text NULL;
