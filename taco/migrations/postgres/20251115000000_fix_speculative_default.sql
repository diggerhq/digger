-- Fix speculative column default to false (normal apply) instead of true (plan-only)
-- Also update existing rows that were created with the wrong default
ALTER TABLE "public"."tfe_configuration_versions" 
  ALTER COLUMN "speculative" SET DEFAULT false;

-- Update all existing configuration versions to be non-speculative
UPDATE "public"."tfe_configuration_versions" 
  SET "speculative" = false 
  WHERE "speculative" = true;

