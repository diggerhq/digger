-- Create index "idx_digger_job_id" to table: "digger_jobs"
CREATE INDEX "idx_digger_job_id" ON "public"."digger_jobs" ("batch_id");
-- Create index "idx_digger_run_batch_id" to table: "digger_run_stages"
CREATE INDEX "idx_digger_run_batch_id" ON "public"."digger_run_stages" ("batch_id");
-- Create "digger_run_queue_items" table
CREATE TABLE "public"."digger_run_queue_items" (
  "id" bigserial NOT NULL,
  "created_at" timestamptz NULL,
  "updated_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "project_id" bigint NULL,
  "digger_run_id" bigint NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_digger_run_queue_items_digger_run" FOREIGN KEY ("digger_run_id") REFERENCES "public"."digger_runs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_digger_run_queue_items_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_digger_run_queue_items_deleted_at" to table: "digger_run_queue_items"
CREATE INDEX "idx_digger_run_queue_items_deleted_at" ON "public"."digger_run_queue_items" ("deleted_at");
-- Create index "idx_digger_run_queue_project_id" to table: "digger_run_queue_items"
CREATE INDEX "idx_digger_run_queue_project_id" ON "public"."digger_run_queue_items" ("project_id");
-- Create index "idx_digger_run_queue_run_id" to table: "digger_run_queue_items"
CREATE INDEX "idx_digger_run_queue_run_id" ON "public"."digger_run_queue_items" ("digger_run_id");
-- Drop "digger_run_queues" table
DROP TABLE "public"."digger_run_queues";
