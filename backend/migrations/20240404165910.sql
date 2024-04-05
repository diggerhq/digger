-- Modify "digger_runs" table
ALTER TABLE "public"."digger_runs" DROP COLUMN "project_id", ADD COLUMN "project_name" text NULL;
