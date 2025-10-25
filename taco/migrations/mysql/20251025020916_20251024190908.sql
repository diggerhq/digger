-- Create "unit_tags" table
CREATE TABLE `unit_tags` (
  `unit_id` varchar(36) NOT NULL,
  `tag_id` varchar(36) NOT NULL,
  PRIMARY KEY (`unit_id`, `tag_id`),
  INDEX `idx_unit_tags_tag_id` (`tag_id`),
  INDEX `idx_unit_tags_unit_id` (`unit_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "permissions" table
CREATE TABLE `permissions` (
  `id` varchar(36) NOT NULL,
  `org_id` varchar(36) NULL,
  `name` varchar(255) NOT NULL,
  `description` text NULL,
  `created_by` text NULL,
  `created_at` datetime NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_permissions_name` (`name`),
  INDEX `idx_permissions_org_id` (`org_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "role_permissions" table
CREATE TABLE `role_permissions` (
  `role_id` varchar(36) NOT NULL,
  `permission_id` varchar(36) NOT NULL,
  PRIMARY KEY (`role_id`, `permission_id`),
  INDEX `idx_role_permissions_permission_id` (`permission_id`),
  INDEX `idx_role_permissions_role_id` (`role_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "roles" table
CREATE TABLE `roles` (
  `id` varchar(36) NOT NULL,
  `org_id` varchar(36) NULL,
  `name` varchar(255) NOT NULL,
  `description` text NULL,
  `created_at` datetime NULL,
  `created_by` text NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_roles_name` (`name`),
  INDEX `idx_roles_org_id` (`org_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "tags" table
CREATE TABLE `tags` (
  `id` varchar(36) NOT NULL,
  `org_id` varchar(36) NULL,
  `name` varchar(255) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_tags_name` (`name`),
  INDEX `idx_tags_org_id` (`org_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "organizations" table
CREATE TABLE `organizations` (
  `id` varchar(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `display_name` varchar(255) NOT NULL,
  `external_org_id` varchar(500) NULL,
  `created_by` varchar(255) NOT NULL,
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_organizations_external_org_id` (`external_org_id`),
  UNIQUE INDEX `idx_organizations_name` (`name`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "units" table
CREATE TABLE `units` (
  `id` varchar(36) NOT NULL,
  `org_id` varchar(36) NULL,
  `name` varchar(255) NOT NULL,
  `size` int NULL DEFAULT 0,
  `updated_at` datetime NULL,
  `locked` bool NULL DEFAULT 0,
  `lock_id` varchar(255) NULL DEFAULT "",
  `lock_who` varchar(255) NULL DEFAULT "",
  `lock_created` datetime NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_units_name` (`name`),
  INDEX `idx_units_org_id` (`org_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "user_roles" table
CREATE TABLE `user_roles` (
  `user_id` varchar(36) NOT NULL,
  `role_id` varchar(36) NOT NULL,
  `org_id` varchar(36) NOT NULL,
  PRIMARY KEY (`user_id`, `role_id`, `org_id`),
  INDEX `idx_user_roles_org_id` (`org_id`),
  INDEX `idx_user_roles_role_id` (`role_id`),
  INDEX `idx_user_roles_user_id` (`user_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "users" table
CREATE TABLE `users` (
  `id` varchar(36) NOT NULL,
  `subject` varchar(255) NOT NULL,
  `email` varchar(255) NOT NULL,
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  `version` int NULL,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_users_email` (`email`),
  UNIQUE INDEX `idx_users_subject` (`subject`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "rules" table
CREATE TABLE `rules` (
  `id` varchar(36) NOT NULL,
  `permission_id` varchar(36) NOT NULL,
  `effect` varchar(8) NOT NULL DEFAULT "allow",
  `wildcard_action` bool NOT NULL DEFAULT 0,
  `wildcard_resource` bool NOT NULL DEFAULT 0,
  `resource_patterns` text NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_rules_permission_id` (`permission_id`),
  CONSTRAINT `fk_permissions_rules` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "rule_actions" table
CREATE TABLE `rule_actions` (
  `id` varchar(36) NOT NULL,
  `rule_id` varchar(36) NOT NULL,
  `action` varchar(128) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_rule_actions_action` (`action`),
  INDEX `idx_rule_actions_rule_id` (`rule_id`),
  CONSTRAINT `fk_rules_actions` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "rule_unit_tags" table
CREATE TABLE `rule_unit_tags` (
  `id` varchar(36) NOT NULL,
  `rule_id` varchar(36) NOT NULL,
  `tag_id` varchar(36) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_rule_unit_tags_rule_id` (`rule_id`),
  INDEX `idx_rule_unit_tags_tag_id` (`tag_id`),
  CONSTRAINT `fk_rules_tag_targets` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "rule_units" table
CREATE TABLE `rule_units` (
  `id` varchar(36) NOT NULL,
  `rule_id` varchar(36) NOT NULL,
  `unit_id` varchar(36) NOT NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_rule_units_rule_id` (`rule_id`),
  INDEX `idx_rule_units_unit_id` (`unit_id`),
  CONSTRAINT `fk_rules_unit_targets` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
