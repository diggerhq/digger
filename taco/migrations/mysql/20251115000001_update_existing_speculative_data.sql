-- Update existing configuration versions and runs that have wrong speculative/plan_only values
-- This fixes data created before we changed the defaults

-- Fix all existing configuration versions
UPDATE `tfe_configuration_versions` 
  SET `speculative` = false 
  WHERE `speculative` = true;

-- Fix all existing runs
UPDATE `tfe_runs`
  SET `plan_only` = false
  WHERE `plan_only` = true;

