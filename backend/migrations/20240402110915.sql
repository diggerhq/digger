-- Drop index "idx_digger_job_id" from table: "digger_jobs"
DROP INDEX "public"."idx_digger_job_id";
-- Create index "idx_digger_job_id" to table: "digger_run_stages"
CREATE INDEX "idx_digger_job_id" ON "public"."digger_run_stages" ("batch_id");
-- Create "digger_run_queues" table
CREATE TABLE "public"."digger_run_queues" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "project_id" bigint NULL,
  "digger_run_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_run_queues_digger_run" FOREIGN KEY ("digger_run_id") REFERENCES "public"."digger_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_digger_run_queues_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_run_queue_project_id" to table: "digger_run_queues"
CREATE INDEX "idx_digger_run_queue_project_id" ON "public"."digger_run_queues" ("project_id");
-- Create index "idx_digger_run_queue_run_id" to table: "digger_run_queues"
CREATE INDEX "idx_digger_run_queue_run_id" ON "public"."digger_run_queues" ("digger_run_id");
-- Create index "idx_digger_run_queues_deleted_at" to table: "digger_run_queues"
CREATE INDEX "idx_digger_run_queues_deleted_at" ON "public"."digger_run_queues" ("deleted_at");
