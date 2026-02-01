-- Remove unique index for active job deduplication
DROP INDEX IF EXISTS idx_job_history_scene_phase_active;

-- Remove pending poll index
DROP INDEX IF EXISTS idx_job_history_pending_poll;

-- Remove priority column
ALTER TABLE job_history DROP COLUMN IF EXISTS priority;
