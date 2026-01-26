-- Add retry fields to job_history
ALTER TABLE job_history
    ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN max_retries INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_retry_at TIMESTAMPTZ,
    ADD COLUMN progress INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN is_retryable BOOLEAN NOT NULL DEFAULT true;

CREATE INDEX idx_job_history_next_retry ON job_history (next_retry_at)
    WHERE next_retry_at IS NOT NULL AND status = 'failed';

-- Dead Letter Queue table
CREATE TABLE dead_letter_queue (
    id BIGSERIAL PRIMARY KEY,
    job_id VARCHAR(36) NOT NULL UNIQUE,
    video_id BIGINT NOT NULL,
    video_title VARCHAR(255) NOT NULL DEFAULT '',
    phase VARCHAR(20) NOT NULL,
    original_error TEXT NOT NULL,
    failure_count INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending_review',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    abandoned_at TIMESTAMPTZ,
    CONSTRAINT valid_dlq_status CHECK (status IN ('pending_review', 'retrying', 'abandoned'))
);

CREATE INDEX idx_dlq_status ON dead_letter_queue (status);
CREATE INDEX idx_dlq_video_id ON dead_letter_queue (video_id);

-- Retry configuration table (per-phase settings)
CREATE TABLE retry_config (
    id SERIAL PRIMARY KEY,
    phase VARCHAR(20) NOT NULL UNIQUE,
    max_retries INTEGER NOT NULL DEFAULT 3,
    initial_delay_seconds INTEGER NOT NULL DEFAULT 30,
    max_delay_seconds INTEGER NOT NULL DEFAULT 3600,
    backoff_factor DECIMAL(3,1) NOT NULL DEFAULT 2.0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_retry_phase CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'scan'))
);

-- Default retry configuration
INSERT INTO retry_config (phase, max_retries, initial_delay_seconds, max_delay_seconds, backoff_factor) VALUES
    ('metadata', 3, 30, 3600, 2.0),
    ('thumbnail', 3, 60, 3600, 2.0),
    ('sprites', 2, 120, 7200, 2.0),
    ('scan', 3, 60, 3600, 2.0);
