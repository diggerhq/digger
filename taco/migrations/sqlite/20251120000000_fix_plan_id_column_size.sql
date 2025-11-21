-- Fix plan ID columns that are too small for TFE-style IDs
-- TFE plan IDs use format: plan-{32chars} = 37 characters total

-- SQLite note: SQLite doesn't enforce varchar lengths
-- No schema changes needed - this file exists only for version consistency

