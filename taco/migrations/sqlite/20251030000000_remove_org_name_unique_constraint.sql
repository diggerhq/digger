-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_organizations" table
CREATE TABLE `new_organizations` (
  `id` varchar NULL,
  `name` varchar NOT NULL,
  `display_name` varchar NOT NULL,
  `external_org_id` varchar NULL,
  `created_by` varchar NOT NULL,
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  PRIMARY KEY (`id`)
);
-- Copy rows from old table "organizations" to new temporary table "new_organizations"
INSERT INTO `new_organizations` (`id`, `name`, `display_name`, `external_org_id`, `created_by`, `created_at`, `updated_at`) SELECT `id`, `name`, `display_name`, `external_org_id`, `created_by`, `created_at`, `updated_at` FROM `organizations`;
-- Drop "organizations" table after copying rows
DROP TABLE `organizations`;
-- Rename temporary table "new_organizations" to "organizations"
ALTER TABLE `new_organizations` RENAME TO `organizations`;
-- Create index "idx_organizations_external_org_id" to table: "organizations"
CREATE UNIQUE INDEX `idx_organizations_external_org_id` ON `organizations` (`external_org_id`);
-- Create index "idx_organizations_name" to table: "organizations"
CREATE INDEX `idx_organizations_name` ON `organizations` (`name`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
