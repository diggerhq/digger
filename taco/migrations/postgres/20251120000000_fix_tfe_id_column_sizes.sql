-- Fix varchar(36) columns that are too small for TFE-style IDs
-- TFE IDs follow patterns like: run-{32chars}, plan-{32chars}, cv-{32chars}
-- Some IDs (plan-) need 37 characters, so we increase to varchar(50) for safety

-- Fix primary key columns
ALTER TABLE "public"."tfe_runs" ALTER COLUMN "id" TYPE varchar(50);
ALTER TABLE "public"."tfe_plans" ALTER COLUMN "id" TYPE varchar(50);
ALTER TABLE "public"."tfe_configuration_versions" ALTER COLUMN "id" TYPE varchar(50);

-- Fix foreign key columns
ALTER TABLE "public"."tfe_runs" ALTER COLUMN "plan_id" TYPE varchar(50);
ALTER TABLE "public"."tfe_runs" ALTER COLUMN "apply_id" TYPE varchar(50);
ALTER TABLE "public"."tfe_runs" ALTER COLUMN "configuration_version_id" TYPE varchar(50);
ALTER TABLE "public"."tfe_plans" ALTER COLUMN "run_id" TYPE varchar(50);
ALTER TABLE "public"."tfe_configuration_versions" ALTER COLUMN "unit_id" TYPE varchar(50);

