-- Create "organizations" table
CREATE TABLE `organizations` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `org_id` varchar NOT NULL,
  `name` varchar NOT NULL,
  `created_by` varchar NOT NULL,
  `created_at` datetime NULL,
  `updated_at` datetime NULL
);
-- Create index "idx_organizations_org_id" to table: "organizations"
CREATE UNIQUE INDEX `idx_organizations_org_id` ON `organizations` (`org_id`);
-- Create "users" table
CREATE TABLE `users` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `subject` varchar NOT NULL,
  `email` varchar NOT NULL,
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  `version` integer NULL
);
-- Create index "idx_users_email" to table: "users"
CREATE UNIQUE INDEX `idx_users_email` ON `users` (`email`);
-- Create index "idx_users_subject" to table: "users"
CREATE UNIQUE INDEX `idx_users_subject` ON `users` (`subject`);
-- Create "user_roles" table
CREATE TABLE `user_roles` (
  `user_id` integer NULL,
  `role_id` integer NULL,
  PRIMARY KEY (`user_id`, `role_id`)
);
-- Create index "idx_user_roles_role_id" to table: "user_roles"
CREATE INDEX `idx_user_roles_role_id` ON `user_roles` (`role_id`);
-- Create index "idx_user_roles_user_id" to table: "user_roles"
CREATE INDEX `idx_user_roles_user_id` ON `user_roles` (`user_id`);
-- Create "roles" table
CREATE TABLE `roles` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `role_id` varchar NOT NULL,
  `name` text NULL,
  `description` text NULL,
  `created_at` datetime NULL,
  `created_by` text NULL
);
-- Create index "idx_roles_role_id" to table: "roles"
CREATE UNIQUE INDEX `idx_roles_role_id` ON `roles` (`role_id`);
-- Create "role_permissions" table
CREATE TABLE `role_permissions` (
  `role_id` integer NULL,
  `permission_id` integer NULL,
  PRIMARY KEY (`role_id`, `permission_id`)
);
-- Create index "idx_role_permissions_permission_id" to table: "role_permissions"
CREATE INDEX `idx_role_permissions_permission_id` ON `role_permissions` (`permission_id`);
-- Create index "idx_role_permissions_role_id" to table: "role_permissions"
CREATE INDEX `idx_role_permissions_role_id` ON `role_permissions` (`role_id`);
-- Create "permissions" table
CREATE TABLE `permissions` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `permission_id` varchar NOT NULL,
  `name` text NULL,
  `description` text NULL,
  `created_by` text NULL,
  `created_at` datetime NULL
);
-- Create index "idx_permissions_permission_id" to table: "permissions"
CREATE UNIQUE INDEX `idx_permissions_permission_id` ON `permissions` (`permission_id`);
-- Create "rules" table
CREATE TABLE `rules` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `permission_id` integer NOT NULL,
  `effect` text NOT NULL DEFAULT 'allow',
  `wildcard_action` numeric NOT NULL DEFAULT false,
  `wildcard_resource` numeric NOT NULL DEFAULT false,
  `resource_patterns` text NULL DEFAULT '',
  CONSTRAINT `fk_permissions_rules` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rules_permission_id" to table: "rules"
CREATE INDEX `idx_rules_permission_id` ON `rules` (`permission_id`);
-- Create "rule_actions" table
CREATE TABLE `rule_actions` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `rule_id` integer NOT NULL,
  `action` text NOT NULL,
  CONSTRAINT `fk_rules_actions` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_actions_action" to table: "rule_actions"
CREATE INDEX `idx_rule_actions_action` ON `rule_actions` (`action`);
-- Create index "idx_rule_actions_rule_id" to table: "rule_actions"
CREATE INDEX `idx_rule_actions_rule_id` ON `rule_actions` (`rule_id`);
-- Create "rule_units" table
CREATE TABLE `rule_units` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `rule_id` integer NOT NULL,
  `unit_id` integer NOT NULL,
  CONSTRAINT `fk_rules_unit_targets` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_units_unit_id" to table: "rule_units"
CREATE INDEX `idx_rule_units_unit_id` ON `rule_units` (`unit_id`);
-- Create index "idx_rule_units_rule_id" to table: "rule_units"
CREATE INDEX `idx_rule_units_rule_id` ON `rule_units` (`rule_id`);
-- Create "rule_unit_tags" table
CREATE TABLE `rule_unit_tags` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `rule_id` integer NOT NULL,
  `tag_id` integer NOT NULL,
  CONSTRAINT `fk_rules_tag_targets` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_rule_unit_tags_tag_id" to table: "rule_unit_tags"
CREATE INDEX `idx_rule_unit_tags_tag_id` ON `rule_unit_tags` (`tag_id`);
-- Create index "idx_rule_unit_tags_rule_id" to table: "rule_unit_tags"
CREATE INDEX `idx_rule_unit_tags_rule_id` ON `rule_unit_tags` (`rule_id`);
-- Create "units" table
CREATE TABLE `units` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `name` varchar NULL,
  `size` integer NULL DEFAULT 0,
  `updated_at` datetime NULL,
  `locked` numeric NULL DEFAULT false,
  `lock_id` text NULL DEFAULT '',
  `lock_who` text NULL DEFAULT '',
  `lock_created` datetime NULL
);
-- Create index "idx_units_name" to table: "units"
CREATE UNIQUE INDEX `idx_units_name` ON `units` (`name`);
-- Create "unit_tags" table
CREATE TABLE `unit_tags` (
  `unit_id` integer NULL,
  `tag_id` integer NULL,
  PRIMARY KEY (`unit_id`, `tag_id`)
);
-- Create index "idx_unit_tags_tag_id" to table: "unit_tags"
CREATE INDEX `idx_unit_tags_tag_id` ON `unit_tags` (`tag_id`);
-- Create index "idx_unit_tags_unit_id" to table: "unit_tags"
CREATE INDEX `idx_unit_tags_unit_id` ON `unit_tags` (`unit_id`);
-- Create "tags" table
CREATE TABLE `tags` (
  `id` integer NULL PRIMARY KEY AUTOINCREMENT,
  `name` varchar NULL
);
-- Create index "idx_tags_name" to table: "tags"
CREATE UNIQUE INDEX `idx_tags_name` ON `tags` (`name`);
