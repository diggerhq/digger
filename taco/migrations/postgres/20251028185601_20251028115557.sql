-- Create "tokens" table
CREATE TABLE "public"."tokens" (
  "id" character varying(36) NOT NULL,
  "user_id" character varying(255) NOT NULL,
  "org_id" character varying(255) NOT NULL,
  "token" character varying(255) NOT NULL,
  "name" character varying(255) NULL,
  "status" character varying(20) NULL DEFAULT 'active',
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "last_used_at" timestamptz NULL,
  "expires_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_tokens_org_id" to table: "tokens"
CREATE INDEX "idx_tokens_org_id" ON "public"."tokens" ("org_id");
-- Create index "idx_tokens_token" to table: "tokens"
CREATE UNIQUE INDEX "idx_tokens_token" ON "public"."tokens" ("token");
-- Create index "idx_tokens_user_id" to table: "tokens"
CREATE INDEX "idx_tokens_user_id" ON "public"."tokens" ("user_id");
