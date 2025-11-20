-- Fix varchar(36) columns that are too small for TFE-style IDs
-- TFE IDs follow patterns like: run-{32chars}, plan-{32chars}, cv-{32chars}
-- Some IDs (plan-) need 37 characters, so we increase to varchar(50) for safety

-- Note: SQLite doesn't enforce varchar lengths - it stores all text as TEXT type
-- The varchar(36) declarations in the original schema are just metadata/hints
-- SQLite will happily store strings longer than the declared length
-- Therefore, no actual schema changes are needed for SQLite

-- This migration file exists for consistency with PostgreSQL/MySQL migrations
-- and to maintain the same migration version across all database types

