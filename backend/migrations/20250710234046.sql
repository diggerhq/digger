-- Modify "projects" table
ALTER TABLE "public"."projects" ADD COLUMN "drift_enabled" boolean NULL DEFAULT false, ADD COLUMN "drift_status" text NULL DEFAULT 'no drift', ADD COLUMN "latest_drift_check" timestamptz NULL, ADD COLUMN "drift_terraform_plan" text NULL;
