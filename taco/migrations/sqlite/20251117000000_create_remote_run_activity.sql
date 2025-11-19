CREATE TABLE IF NOT EXISTS remote_run_activity (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  org_id TEXT NOT NULL,
  unit_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  triggered_by TEXT,
  triggered_source TEXT,
  sandbox_provider TEXT,
  sandbox_job_id TEXT,
  started_at DATETIME,
  completed_at DATETIME,
  duration_ms INTEGER,
  error_message TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (run_id) REFERENCES tfe_runs(id) ON DELETE CASCADE,
  FOREIGN KEY (unit_id) REFERENCES units(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_remote_runs_run_id ON remote_run_activity (run_id);
CREATE INDEX IF NOT EXISTS idx_remote_runs_unit_id ON remote_run_activity (unit_id);
CREATE INDEX IF NOT EXISTS idx_remote_runs_created_at ON remote_run_activity (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_remote_runs_operation ON remote_run_activity (operation);

