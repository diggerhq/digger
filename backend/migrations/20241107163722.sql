-- Create "github_app_connections" table
CREATE TABLE "public"."github_app_connections" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "github_id" bigint NULL,
  "client_id" text NULL,
  "client_secret_encrypted" text NULL,
  "webhook_secret_encrypted" text NULL,
  "private_key_encrypted" text NULL,
  "private_key_base64_encrypted" text NULL,
  "org" text NULL,
  "name" text NULL,
  "github_app_url" text NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_github_app_connections_deleted_at" to table: "github_app_connections"
CREATE INDEX "idx_github_app_connections_deleted_at" ON "public"."github_app_connections" ("deleted_at");
-- Drop "github_apps" table
DROP TABLE "public"."github_apps";
