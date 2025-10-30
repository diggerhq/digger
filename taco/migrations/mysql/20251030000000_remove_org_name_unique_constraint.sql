-- Modify "organizations" table
ALTER TABLE `organizations` DROP INDEX `idx_organizations_name`;
-- Modify "organizations" table
ALTER TABLE `organizations` ADD INDEX `idx_organizations_name` (`name`);
