-- Update existing configuration versions and runs that have wrong speculative/plan_only values
-- This fixes data created before we changed the defaults

-- Fix all existing configuration versions
UPDATE `tfe_configuration_versions` 
  SET `speculative` = 0 
  WHERE `speculative` = 1;

-- Fix all existing runs
UPDATE `tfe_runs`
  SET `plan_only` = 0
  WHERE `plan_only` = 1;

