-- Modify "github_app_connections" table
ALTER TABLE "public"."github_app_connections" ADD COLUMN "gitlab_access_token_encrypted" text NULL, ADD COLUMN "gitlab_webhook_secret_encrypted" text NULL;
