-- Add TFE-specific fields to units table
ALTER TABLE `units` ADD COLUMN `tfe_auto_apply` integer DEFAULT NULL;
ALTER TABLE `units` ADD COLUMN `tfe_terraform_version` varchar(50) DEFAULT NULL;
ALTER TABLE `units` ADD COLUMN `tfe_working_directory` varchar(500) DEFAULT NULL;
ALTER TABLE `units` ADD COLUMN `tfe_execution_mode` varchar(50) DEFAULT NULL;

-- Create tfe_runs table
CREATE TABLE IF NOT EXISTS `tfe_runs` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `is_destroy` integer NOT NULL DEFAULT 0,
  `message` text,
  `plan_only` integer NOT NULL DEFAULT 1,
  `source` varchar(50) NOT NULL DEFAULT 'cli',
  `is_cancelable` integer NOT NULL DEFAULT 1,
  `can_apply` integer NOT NULL DEFAULT 0,
  `configuration_version_id` varchar(36) NOT NULL,
  `plan_id` varchar(36),
  `apply_id` varchar(36),
  `created_by` varchar(255),
  FOREIGN KEY (`unit_id`) REFERENCES `units` (`id`) ON DELETE CASCADE,
  FOREIGN KEY (`configuration_version_id`) REFERENCES `tfe_configuration_versions` (`id`) ON DELETE CASCADE
);

-- Create indexes for tfe_runs
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_org_id` ON `tfe_runs` (`org_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_unit_id` ON `tfe_runs` (`unit_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_configuration_version_id` ON `tfe_runs` (`configuration_version_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_plan_id` ON `tfe_runs` (`plan_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_status` ON `tfe_runs` (`status`);
CREATE INDEX IF NOT EXISTS `idx_tfe_runs_created_at` ON `tfe_runs` (`created_at` DESC);

-- Create tfe_plans table
CREATE TABLE IF NOT EXISTS `tfe_plans` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `run_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `resource_additions` integer NOT NULL DEFAULT 0,
  `resource_changes` integer NOT NULL DEFAULT 0,
  `resource_destructions` integer NOT NULL DEFAULT 0,
  `has_changes` integer NOT NULL DEFAULT 0,
  `log_blob_id` varchar(255),
  `log_read_url` text,
  `plan_output_blob_id` varchar(255),
  `plan_output_json` text,
  `created_by` varchar(255),
  FOREIGN KEY (`run_id`) REFERENCES `tfe_runs` (`id`) ON DELETE CASCADE
);

-- Create indexes for tfe_plans
CREATE INDEX IF NOT EXISTS `idx_tfe_plans_org_id` ON `tfe_plans` (`org_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_plans_run_id` ON `tfe_plans` (`run_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_plans_status` ON `tfe_plans` (`status`);

-- Create tfe_configuration_versions table
CREATE TABLE IF NOT EXISTS `tfe_configuration_versions` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `source` varchar(50) NOT NULL DEFAULT 'cli',
  `speculative` integer NOT NULL DEFAULT 1,
  `auto_queue_runs` integer NOT NULL DEFAULT 0,
  `provisional` integer NOT NULL DEFAULT 0,
  `error` text,
  `error_message` text,
  `upload_url` text,
  `uploaded_at` datetime,
  `archive_blob_id` varchar(255),
  `status_timestamps` text NOT NULL DEFAULT '{}',
  `created_by` varchar(255),
  FOREIGN KEY (`unit_id`) REFERENCES `units` (`id`) ON DELETE CASCADE
);

-- Create indexes for tfe_configuration_versions
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_org_id` ON `tfe_configuration_versions` (`org_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_unit_id` ON `tfe_configuration_versions` (`unit_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_status` ON `tfe_configuration_versions` (`status`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_created_at` ON `tfe_configuration_versions` (`created_at` DESC);

