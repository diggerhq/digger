-- Create "repo_caches" table
CREATE TABLE "public"."repo_caches" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "org_id" bigint NULL,
  "repo_full_name" text NULL,
  "digger_yml_str" text NULL,
  "digger_config" bytea NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_repo_caches_deleted_at" to table: "repo_caches"
CREATE INDEX "idx_repo_caches_deleted_at" ON "public"."repo_caches" ("deleted_at");
