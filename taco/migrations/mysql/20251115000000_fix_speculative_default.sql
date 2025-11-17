-- Fix speculative column default to false (normal apply) instead of true (plan-only)
-- Also update existing rows that were created with the wrong default
ALTER TABLE `tfe_configuration_versions` 
  MODIFY COLUMN `speculative` boolean NOT NULL DEFAULT false;

-- Update all existing configuration versions to be non-speculative
UPDATE `tfe_configuration_versions` 
  SET `speculative` = false 
  WHERE `speculative` = true;

