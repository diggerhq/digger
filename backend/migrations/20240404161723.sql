-- Modify "digger_runs" table
ALTER TABLE "public"."digger_runs" ADD COLUMN "plan_stage_id" bigint NULL, ADD COLUMN "apply_stage_id" bigint NULL, ADD
 CONSTRAINT "fk_digger_runs_apply_stage" FOREIGN KEY ("apply_stage_id") REFERENCES "public"."digger_run_stages" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD
 CONSTRAINT "fk_digger_runs_plan_stage" FOREIGN KEY ("plan_stage_id") REFERENCES "public"."digger_run_stages" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
