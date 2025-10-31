-- Add unique constraint on (org_id, name) for units table
-- This ensures unit names are unique within each organization

-- Create unique index on org_id + name
CREATE UNIQUE INDEX "idx_units_org_name" ON "public"."units" ("org_id", "name");

