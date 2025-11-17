-- Fix speculative column default to 0/false (normal apply) instead of 1/true (plan-only)
-- SQLite doesn't support ALTER COLUMN, so we need to recreate the table

-- Step 1: Create new table with correct default
CREATE TABLE IF NOT EXISTS `tfe_configuration_versions_new` (
  `id` varchar(36) NOT NULL PRIMARY KEY,
  `org_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `status` varchar(50) NOT NULL DEFAULT 'pending',
  `source` varchar(50) NOT NULL DEFAULT 'cli',
  `speculative` integer NOT NULL DEFAULT 0,
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

-- Step 2: Copy data from old table, forcing speculative to 0 (false)
INSERT INTO `tfe_configuration_versions_new` 
  (`id`, `org_id`, `unit_id`, `created_at`, `updated_at`, `status`, `source`, 
   `speculative`, `auto_queue_runs`, `provisional`, `error`, `error_message`, 
   `upload_url`, `uploaded_at`, `archive_blob_id`, `status_timestamps`, `created_by`)
SELECT 
  `id`, `org_id`, `unit_id`, `created_at`, `updated_at`, `status`, `source`, 
  0 AS `speculative`,  -- Force all to non-speculative
  `auto_queue_runs`, `provisional`, `error`, `error_message`, 
  `upload_url`, `uploaded_at`, `archive_blob_id`, `status_timestamps`, `created_by`
FROM `tfe_configuration_versions`;

-- Step 3: Drop old table
DROP TABLE `tfe_configuration_versions`;

-- Step 4: Rename new table
ALTER TABLE `tfe_configuration_versions_new` RENAME TO `tfe_configuration_versions`;

-- Step 5: Recreate indexes
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_org_id` ON `tfe_configuration_versions` (`org_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_unit_id` ON `tfe_configuration_versions` (`unit_id`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_status` ON `tfe_configuration_versions` (`status`);
CREATE INDEX IF NOT EXISTS `idx_tfe_configuration_versions_created_at` ON `tfe_configuration_versions` (`created_at` DESC);

