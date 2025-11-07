-- Add composite index for efficient unit queries by org and name
-- This covers the common pattern: WHERE org_id = ? AND name LIKE ?%
CREATE INDEX `idx_units_org_id_name` ON `units` (`org_id`, `name`);

-- Add composite index for role lookups by org and name
CREATE INDEX `idx_roles_org_id_name` ON `roles` (`org_id`, `name`);

-- Add composite index for permission lookups by org and name  
CREATE INDEX `idx_permissions_org_id_name` ON `permissions` (`org_id`, `name`);

-- Add composite index for tag lookups by org and name
CREATE INDEX `idx_tags_org_id_name` ON `tags` (`org_id`, `name`);

-- Note: These composite indexes dramatically speed up queries like:
--   SELECT * FROM units WHERE org_id = 'uuid' AND name LIKE 'prefix%';
--   SELECT id FROM roles WHERE org_id = 'uuid' AND name = 'admin';
-- MySQL can use these indexes for both exact matches and prefix searches.

