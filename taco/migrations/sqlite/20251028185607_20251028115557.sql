-- Create "tokens" table
CREATE TABLE `tokens` (
  `id` varchar NULL,
  `user_id` varchar NOT NULL,
  `org_id` varchar NOT NULL,
  `token` varchar NOT NULL,
  `name` varchar NULL,
  `status` varchar NULL DEFAULT 'active',
  `created_at` datetime NULL,
  `updated_at` datetime NULL,
  `last_used_at` datetime NULL,
  `expires_at` datetime NULL,
  PRIMARY KEY (`id`)
);
-- Create index "idx_tokens_token" to table: "tokens"
CREATE UNIQUE INDEX `idx_tokens_token` ON `tokens` (`token`);
-- Create index "idx_tokens_org_id" to table: "tokens"
CREATE INDEX `idx_tokens_org_id` ON `tokens` (`org_id`);
-- Create index "idx_tokens_user_id" to table: "tokens"
CREATE INDEX `idx_tokens_user_id` ON `tokens` (`user_id`);
