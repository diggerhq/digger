-- Add composite index for efficient unit queries by org and name
-- This covers the common pattern: WHERE org_id = ? AND name LIKE ?%
CREATE INDEX IF NOT EXISTS "idx_units_org_id_name" ON "public"."units" ("org_id", "name");

-- Add composite index for role lookups by org and name
CREATE INDEX IF NOT EXISTS "idx_roles_org_id_name" ON "public"."roles" ("org_id", "name");

-- Add composite index for permission lookups by org and name  
CREATE INDEX IF NOT EXISTS "idx_permissions_org_id_name" ON "public"."permissions" ("org_id", "name");

-- Add composite index for tag lookups by org and name
CREATE INDEX IF NOT EXISTS "idx_tags_org_id_name" ON "public"."tags" ("org_id", "name");

-- Note: These composite indexes dramatically speed up queries like:
--   SELECT * FROM units WHERE org_id = 'uuid' AND name LIKE 'prefix%';
--   SELECT id FROM roles WHERE org_id = 'uuid' AND name = 'admin';
-- PostgreSQL can use these indexes for both exact matches and prefix searches.

