-- Add TFE-specific fields to units table
ALTER TABLE `units` 
  ADD COLUMN `tfe_auto_apply` boolean DEFAULT NULL,
  ADD COLUMN `tfe_terraform_version` varchar(50) DEFAULT NULL,
  ADD COLUMN `tfe_working_directory` varchar(500) DEFAULT NULL,
  ADD COLUMN `tfe_execution_mode` varchar(50) DEFAULT NULL;

-- Create tfe_runs table
CREATE TABLE IF NOT EXISTS `tfe_runs` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `is_destroy` boolean NOT NULL DEFAULT false,
  `message` text,
  `plan_only` boolean NOT NULL DEFAULT true,
  `source` varchar(50) NOT NULL DEFAULT 'cli',
  `is_cancelable` boolean NOT NULL DEFAULT true,
  `can_apply` boolean NOT NULL DEFAULT false,
  `configuration_version_id` varchar(36) NOT NULL,
  `plan_id` varchar(36),
  `apply_id` varchar(36),
  `created_by` varchar(255),
  INDEX `idx_tfe_runs_org_id` (`org_id`),
  INDEX `idx_tfe_runs_unit_id` (`unit_id`),
  INDEX `idx_tfe_runs_configuration_version_id` (`configuration_version_id`),
  INDEX `idx_tfe_runs_plan_id` (`plan_id`),
  INDEX `idx_tfe_runs_status` (`status`),
  INDEX `idx_tfe_runs_created_at` (`created_at` DESC)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Create tfe_plans table
CREATE TABLE IF NOT EXISTS `tfe_plans` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `run_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `resource_additions` int NOT NULL DEFAULT 0,
  `resource_changes` int NOT NULL DEFAULT 0,
  `resource_destructions` int NOT NULL DEFAULT 0,
  `has_changes` boolean NOT NULL DEFAULT false,
  `log_blob_id` varchar(255),
  `log_read_url` text,
  `plan_output_blob_id` varchar(255),
  `plan_output_json` longtext,
  `created_by` varchar(255),
  INDEX `idx_tfe_plans_org_id` (`org_id`),
  INDEX `idx_tfe_plans_run_id` (`run_id`),
  INDEX `idx_tfe_plans_status` (`status`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Create tfe_configuration_versions table
CREATE TABLE IF NOT EXISTS `tfe_configuration_versions` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `source` varchar(50) NOT NULL DEFAULT 'cli',
  `speculative` boolean NOT NULL DEFAULT true,
  `auto_queue_runs` boolean NOT NULL DEFAULT false,
  `provisional` boolean NOT NULL DEFAULT false,
  `error` text,
  `error_message` text,
  `upload_url` text,
  `uploaded_at` datetime,
  `archive_blob_id` varchar(255),
  `status_timestamps` json NOT NULL,
  `created_by` varchar(255),
  INDEX `idx_tfe_configuration_versions_org_id` (`org_id`),
  INDEX `idx_tfe_configuration_versions_unit_id` (`unit_id`),
  INDEX `idx_tfe_configuration_versions_status` (`status`),
  INDEX `idx_tfe_configuration_versions_created_at` (`created_at` DESC)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Add foreign key constraints
ALTER TABLE `tfe_runs` 
  ADD CONSTRAINT `fk_tfe_runs_unit` 
  FOREIGN KEY (`unit_id`) REFERENCES `units` (`id`) ON DELETE CASCADE;

ALTER TABLE `tfe_runs` 
  ADD CONSTRAINT `fk_tfe_runs_configuration_version` 
  FOREIGN KEY (`configuration_version_id`) REFERENCES `tfe_configuration_versions` (`id`) ON DELETE CASCADE;

ALTER TABLE `tfe_plans` 
  ADD CONSTRAINT `fk_tfe_plans_run` 
  FOREIGN KEY (`run_id`) REFERENCES `tfe_runs` (`id`) ON DELETE CASCADE;

ALTER TABLE `tfe_configuration_versions` 
  ADD CONSTRAINT `fk_tfe_configuration_versions_unit` 
  FOREIGN KEY (`unit_id`) REFERENCES `units` (`id`) ON DELETE CASCADE;

