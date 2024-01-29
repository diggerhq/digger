-- Create "digger_job_summaries" table
CREATE TABLE "public"."digger_job_summaries" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "resources_created" bigint NULL,
  "resources_deleted" bigint NULL,
  "resources_updated" bigint NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_digger_job_summaries_deleted_at" to table: "digger_job_summaries"
CREATE INDEX "idx_digger_job_summaries_deleted_at" ON "public"."digger_job_summaries" ("deleted_at");
-- Modify "digger_jobs" table
ALTER TABLE "public"."digger_jobs" ADD COLUMN "digger_job_summary_id" bigint NULL, ADD
 CONSTRAINT "fk_digger_jobs_digger_job_summary" FOREIGN KEY ("digger_job_summary_id") REFERENCES "public"."digger_job_summaries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
