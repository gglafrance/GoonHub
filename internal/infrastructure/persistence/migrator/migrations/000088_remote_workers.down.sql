DROP TABLE IF EXISTS worker_settings;

DROP INDEX IF EXISTS idx_job_history_worker_id;
DROP INDEX IF EXISTS idx_job_history_remote_lease_expiry;

ALTER TABLE job_history
    DROP COLUMN IF EXISTS lease_params,
    DROP COLUMN IF EXISTS worker_stage,
    DROP COLUMN IF EXISTS input_mode,
    DROP COLUMN IF EXISTS commit_started_generation,
    DROP COLUMN IF EXISTS committed_generation,
    DROP COLUMN IF EXISTS last_heartbeat_at,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS lease_generation,
    DROP COLUMN IF EXISTS worker_id;

DROP TABLE IF EXISTS workers;
