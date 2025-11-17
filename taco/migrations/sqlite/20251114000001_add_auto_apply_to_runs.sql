-- Add auto_apply and apply_log_blob_id columns to tfe_runs table

ALTER TABLE tfe_runs ADD COLUMN auto_apply BOOLEAN DEFAULT FALSE;
ALTER TABLE tfe_runs ADD COLUMN apply_log_blob_id VARCHAR(255);

