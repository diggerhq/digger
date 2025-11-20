-- Fix plan ID columns that are too small for TFE-style IDs
-- TFE plan IDs use format: plan-{32chars} = 37 characters total
-- Original varchar(36) is too small and causes INSERT errors

-- Fix tfe_plans.id (primary key)
ALTER TABLE `tfe_plans`
  MODIFY COLUMN `id` varchar(50) NOT NULL;

-- Fix tfe_runs.plan_id (foreign key reference)
ALTER TABLE `tfe_runs`
  MODIFY COLUMN `plan_id` varchar(50);

