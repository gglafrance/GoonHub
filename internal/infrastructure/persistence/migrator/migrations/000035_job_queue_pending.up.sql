-- Add priority column for job ordering support
ALTER TABLE job_history ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;

-- Efficient polling: fetch pending jobs by phase, ordered by priority then FIFO
CREATE INDEX idx_job_history_pending_poll
    ON job_history (phase, priority DESC, created_at ASC)
    WHERE status = 'pending';

-- Prevent duplicate pending/running jobs for same scene+phase
CREATE UNIQUE INDEX idx_job_history_scene_phase_active
    ON job_history (scene_id, phase)
    WHERE status IN ('pending', 'running');
