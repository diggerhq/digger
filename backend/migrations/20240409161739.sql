-- Create "job_tokens" table
CREATE TABLE "public"."job_tokens" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "value" text NULL,
  "expiry" timestamptz NULL,
  "organisation_id" bigint NULL,
  "type" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_job_tokens_organisation" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_job_tokens_deleted_at" to table: "job_tokens"
CREATE INDEX "idx_job_tokens_deleted_at" ON "public"."job_tokens" ("deleted_at");
