-- Add engine field to support both Terraform and OpenTofu
ALTER TABLE units 
ADD COLUMN tfe_engine TEXT DEFAULT 'terraform';

-- Add index for engine queries
CREATE INDEX idx_units_tfe_engine ON units (tfe_engine);

