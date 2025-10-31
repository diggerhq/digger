-- Add unique constraints on (org_id, name) for roles, permissions, and tags tables
-- This ensures names are unique within each organization

-- Create unique index on roles (org_id, name)
CREATE UNIQUE INDEX `unique_org_role_name` ON `roles` (`org_id`, `name`);

-- Create unique index on permissions (org_id, name)
CREATE UNIQUE INDEX `unique_org_permission_name` ON `permissions` (`org_id`, `name`);

-- Create unique index on tags (org_id, name)
CREATE UNIQUE INDEX `unique_org_tag_name` ON `tags` (`org_id`, `name`);

