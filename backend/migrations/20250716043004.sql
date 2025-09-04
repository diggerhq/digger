-- Modify "digger_jobs" table
ALTER TABLE "public"."digger_jobs" ADD COLUMN "run_name" text NULL, ADD COLUMN "project_name" text NULL, ADD COLUMN "serialized_reporter_spec" bytea NULL, ADD COLUMN "serialized_comment_updater_spec" bytea NULL, ADD COLUMN "serialized_lock_spec" bytea NULL, ADD COLUMN "serialized_backend_spec" bytea NULL, ADD COLUMN "serialized_vcs_spec" bytea NULL, ADD COLUMN "serialized_policy_spec" bytea NULL, ADD COLUMN "serialized_variables_spec" bytea NULL;
-- Modify "organisations" table
ALTER TABLE "public"."organisations" ADD COLUMN "slack_notification_url" text NULL;
-- Modify "projects" table
ALTER TABLE "public"."projects" ADD COLUMN "drift_to_create" bigint NULL, ADD COLUMN "drift_to_update" bigint NULL, ADD COLUMN "drift_to_delete" bigint NULL;
-- Modify "repos" table
ALTER TABLE "public"."repos" ADD COLUMN "github_installation_id" text NULL, ADD COLUMN "github_app_id" bigint NULL, ADD COLUMN "default_branch" text NULL, ADD COLUMN "clone_url" text NULL;
