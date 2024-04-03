-- Modify "digger_run_stages" table
ALTER TABLE "public"."digger_run_stages" DROP COLUMN "digger_run_stage_id", DROP COLUMN "project_name", DROP COLUMN "status", DROP COLUMN "digger_job_summary_id", DROP COLUMN "serialized_job_spec", DROP COLUMN "workflow_file", DROP COLUMN "workflow_run_url", ADD COLUMN "batch_id" text NULL, ADD
 CONSTRAINT "fk_digger_run_stages_batch" FOREIGN KEY ("batch_id") REFERENCES "public"."digger_batches" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
