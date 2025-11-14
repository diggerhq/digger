-- Add TFE-specific fields to units table
ALTER TABLE "public"."units" 
  ADD COLUMN IF NOT EXISTS "tfe_auto_apply" boolean DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS "tfe_terraform_version" varchar(50) DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS "tfe_working_directory" varchar(500) DEFAULT NULL,
  ADD COLUMN IF NOT EXISTS "tfe_execution_mode" varchar(50) DEFAULT NULL;

-- Create tfe_runs table
CREATE TABLE IF NOT EXISTS "public"."tfe_runs" (
  "id" varchar(36) NOT NULL PRIMARY KEY,
  "org_id" varchar(36) NOT NULL,
  "unit_id" varchar(36) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  "status" varchar(50) NOT NULL DEFAULT 'pending',
  "is_destroy" boolean NOT NULL DEFAULT false,
  "message" text,
  "plan_only" boolean NOT NULL DEFAULT true,
  "source" varchar(50) NOT NULL DEFAULT 'cli',
  "is_cancelable" boolean NOT NULL DEFAULT true,
  "can_apply" boolean NOT NULL DEFAULT false,
  "configuration_version_id" varchar(36) NOT NULL,
  "plan_id" varchar(36),
  "apply_id" varchar(36),
  "created_by" varchar(255)
);

-- Create indexes for tfe_runs
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_org_id" ON "public"."tfe_runs" ("org_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_unit_id" ON "public"."tfe_runs" ("unit_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_configuration_version_id" ON "public"."tfe_runs" ("configuration_version_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_plan_id" ON "public"."tfe_runs" ("plan_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_status" ON "public"."tfe_runs" ("status");
CREATE INDEX IF NOT EXISTS "idx_tfe_runs_created_at" ON "public"."tfe_runs" ("created_at" DESC);

-- Create tfe_plans table
CREATE TABLE IF NOT EXISTS "public"."tfe_plans" (
  "id" varchar(36) NOT NULL PRIMARY KEY,
  "org_id" varchar(36) NOT NULL,
  "run_id" varchar(36) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  "status" varchar(50) NOT NULL DEFAULT 'pending',
  "resource_additions" integer NOT NULL DEFAULT 0,
  "resource_changes" integer NOT NULL DEFAULT 0,
  "resource_destructions" integer NOT NULL DEFAULT 0,
  "has_changes" boolean NOT NULL DEFAULT false,
  "log_blob_id" varchar(255),
  "log_read_url" text,
  "plan_output_blob_id" varchar(255),
  "plan_output_json" text,
  "created_by" varchar(255)
);

-- Create indexes for tfe_plans
CREATE INDEX IF NOT EXISTS "idx_tfe_plans_org_id" ON "public"."tfe_plans" ("org_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_plans_run_id" ON "public"."tfe_plans" ("run_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_plans_status" ON "public"."tfe_plans" ("status");

-- Create tfe_configuration_versions table
CREATE TABLE IF NOT EXISTS "public"."tfe_configuration_versions" (
  "id" varchar(36) NOT NULL PRIMARY KEY,
  "org_id" varchar(36) NOT NULL,
  "unit_id" varchar(36) NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT now(),
  "updated_at" timestamp NOT NULL DEFAULT now(),
  "status" varchar(50) NOT NULL DEFAULT 'pending',
  "source" varchar(50) NOT NULL DEFAULT 'cli',
  "speculative" boolean NOT NULL DEFAULT true,
  "auto_queue_runs" boolean NOT NULL DEFAULT false,
  "provisional" boolean NOT NULL DEFAULT false,
  "error" text,
  "error_message" text,
  "upload_url" text,
  "uploaded_at" timestamp,
  "archive_blob_id" varchar(255),
  "status_timestamps" json NOT NULL DEFAULT '{}',
  "created_by" varchar(255)
);

-- Create indexes for tfe_configuration_versions
CREATE INDEX IF NOT EXISTS "idx_tfe_configuration_versions_org_id" ON "public"."tfe_configuration_versions" ("org_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_configuration_versions_unit_id" ON "public"."tfe_configuration_versions" ("unit_id");
CREATE INDEX IF NOT EXISTS "idx_tfe_configuration_versions_status" ON "public"."tfe_configuration_versions" ("status");
CREATE INDEX IF NOT EXISTS "idx_tfe_configuration_versions_created_at" ON "public"."tfe_configuration_versions" ("created_at" DESC);

-- Add foreign key constraints (optional - for referential integrity)
ALTER TABLE "public"."tfe_runs" 
  ADD CONSTRAINT IF NOT EXISTS "fk_tfe_runs_unit" 
  FOREIGN KEY ("unit_id") REFERENCES "public"."units" ("id") ON DELETE CASCADE;

ALTER TABLE "public"."tfe_runs" 
  ADD CONSTRAINT IF NOT EXISTS "fk_tfe_runs_configuration_version" 
  FOREIGN KEY ("configuration_version_id") REFERENCES "public"."tfe_configuration_versions" ("id") ON DELETE CASCADE;

ALTER TABLE "public"."tfe_plans" 
  ADD CONSTRAINT IF NOT EXISTS "fk_tfe_plans_run" 
  FOREIGN KEY ("run_id") REFERENCES "public"."tfe_runs" ("id") ON DELETE CASCADE;

ALTER TABLE "public"."tfe_configuration_versions" 
  ADD CONSTRAINT IF NOT EXISTS "fk_tfe_configuration_versions_unit" 
  FOREIGN KEY ("unit_id") REFERENCES "public"."units" ("id") ON DELETE CASCADE;

