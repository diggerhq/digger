CREATE TABLE IF NOT EXISTS public.remote_run_activity (
    id uuid PRIMARY KEY,
    run_id varchar(36) NOT NULL REFERENCES public.tfe_runs(id) ON DELETE CASCADE,
    org_id varchar(36) NOT NULL,
    unit_id varchar(36) NOT NULL REFERENCES public.units(id) ON DELETE CASCADE,
    operation varchar(16) NOT NULL,
    status varchar(32) NOT NULL DEFAULT 'pending',
    triggered_by varchar(255),
    triggered_source varchar(50),
    sandbox_provider varchar(50),
    sandbox_job_id varchar(100),
    started_at timestamptz,
    completed_at timestamptz,
    duration_ms bigint,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_remote_runs_run_id ON public.remote_run_activity (run_id);
CREATE INDEX IF NOT EXISTS idx_remote_runs_unit_id ON public.remote_run_activity (unit_id);
CREATE INDEX IF NOT EXISTS idx_remote_runs_created_at ON public.remote_run_activity (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_remote_runs_operation ON public.remote_run_activity (operation);

CREATE OR REPLACE FUNCTION public.remote_run_activity_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS remote_run_activity_set_updated_at ON public.remote_run_activity;
CREATE TRIGGER remote_run_activity_set_updated_at
BEFORE UPDATE ON public.remote_run_activity
FOR EACH ROW EXECUTE PROCEDURE public.remote_run_activity_set_updated_at();

