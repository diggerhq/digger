-- Create "job_artefacts" table
CREATE TABLE "public"."job_artefacts" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "job_token_id" bigint NULL,
  "contents" bytea NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_job_artefacts_job_token" FOREIGN KEY ("job_token_id") REFERENCES "public"."job_tokens" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_job_artefacts_deleted_at" to table: "job_artefacts"
CREATE INDEX "idx_job_artefacts_deleted_at" ON "public"."job_artefacts" ("deleted_at");
