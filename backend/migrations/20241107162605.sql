-- Modify "github_apps" table
ALTER TABLE "public"."github_apps" ADD COLUMN "client_id" text NULL, ADD COLUMN "client_secret_encrypted" text NULL, ADD COLUMN "webhook_secret_encrypted" text NULL, ADD COLUMN "private_key_encrypted" text NULL, ADD COLUMN "private_key_base64_encrypted" text NULL, ADD COLUMN "org" text NULL;
