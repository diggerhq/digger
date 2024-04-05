-- Modify "digger_runs" table
ALTER TABLE "public"."digger_runs" ADD COLUMN "is_approved" boolean NULL, ADD COLUMN "approval_author" text NULL, ADD COLUMN "approval_date" timestamptz NULL;
