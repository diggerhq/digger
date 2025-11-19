-- Add engine field to support both Terraform and OpenTofu
ALTER TABLE `units` 
ADD COLUMN `tfe_engine` VARCHAR(20) DEFAULT 'terraform' COMMENT 'IaC engine: terraform or tofu';

-- Add index for engine queries
CREATE INDEX `idx_units_tfe_engine` ON `units` (`tfe_engine`);

