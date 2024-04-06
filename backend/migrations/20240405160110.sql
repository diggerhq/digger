-- Modify "digger_run_queue_items" table
ALTER TABLE "public"."digger_run_queue_items" ADD COLUMN "project_id" bigint NULL, ADD
 CONSTRAINT "fk_digger_run_queue_items_project" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
