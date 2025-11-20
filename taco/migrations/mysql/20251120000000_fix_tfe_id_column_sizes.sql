-- Fix varchar(36) columns that are too small for TFE-style IDs
-- TFE IDs follow patterns like: run-{32chars}, plan-{32chars}, cv-{32chars}
-- Some IDs (plan-) need 37 characters, so we increase to varchar(50) for safety

-- Fix primary key columns
ALTER TABLE `tfe_runs` MODIFY COLUMN `id` varchar(50) NOT NULL;
ALTER TABLE `tfe_plans` MODIFY COLUMN `id` varchar(50) NOT NULL;
ALTER TABLE `tfe_configuration_versions` MODIFY COLUMN `id` varchar(50) NOT NULL;

-- Fix foreign key columns
ALTER TABLE `tfe_runs` MODIFY COLUMN `plan_id` varchar(50);
ALTER TABLE `tfe_runs` MODIFY COLUMN `apply_id` varchar(50);
ALTER TABLE `tfe_runs` MODIFY COLUMN `configuration_version_id` varchar(50) NOT NULL;
ALTER TABLE `tfe_plans` MODIFY COLUMN `run_id` varchar(50) NOT NULL;
ALTER TABLE `tfe_configuration_versions` MODIFY COLUMN `unit_id` varchar(50) NOT NULL;

