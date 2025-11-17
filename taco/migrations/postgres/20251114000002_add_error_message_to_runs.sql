-- Add error_message column to tfe_runs table

ALTER TABLE "public"."tfe_runs" ADD COLUMN "error_message" TEXT;

