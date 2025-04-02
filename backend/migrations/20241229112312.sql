-- Modify "digger_batches" table
ALTER TABLE "public"."digger_batches" ADD COLUMN "ai_summary_comment_id" text NULL,
ADD COLUMN "report_terraform_outputs" boolean NULL;
