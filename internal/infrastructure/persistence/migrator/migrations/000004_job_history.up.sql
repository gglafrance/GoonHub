CREATE TABLE job_history (
    id BIGSERIAL PRIMARY KEY,
    job_id VARCHAR(36) NOT NULL,
    video_id BIGINT NOT NULL,
    video_title VARCHAR(255) NOT NULL DEFAULT '',
    phase VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_job_history_job_id ON job_history (job_id);
CREATE INDEX idx_job_history_started_at ON job_history (started_at);
CREATE INDEX idx_job_history_status ON job_history (status);
