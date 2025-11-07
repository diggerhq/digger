-- Create "tokens" table
CREATE TABLE `tokens` (
  `id` varchar(36) NOT NULL,
  `user_id` varchar(255) NOT NULL,
  `org_id` varchar(255) NOT NULL,
  `token` varchar(255) NOT NULL,
  `name` varchar(255) NULL,
  `status` varchar(20) NULL DEFAULT "active",
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  `last_used_at` datetime NULL,
  `expires_at` datetime NULL,
  PRIMARY KEY (`id`),
  INDEX `idx_tokens_org_id` (`org_id`),
  UNIQUE INDEX `idx_tokens_token` (`token`),
  INDEX `idx_tokens_user_id` (`user_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
