DROP TABLE IF EXISTS retry_config;
DROP TABLE IF EXISTS dead_letter_queue;
DROP INDEX IF EXISTS idx_job_history_next_retry;
ALTER TABLE job_history
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS max_retries,
    DROP COLUMN IF EXISTS next_retry_at,
    DROP COLUMN IF EXISTS progress,
    DROP COLUMN IF EXISTS is_retryable;
