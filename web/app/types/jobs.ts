export interface JobHistory {
    id: number;
    job_id: string;
    video_id: number;
    video_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites';
    status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'timed_out';
    error_message?: string;
    started_at: string;
    completed_at?: string;
    created_at: string;
    retry_count: number;
    max_retries: number;
    next_retry_at?: string;
    progress: number;
    is_retryable: boolean;
    priority: number;
}

export interface PoolConfig {
    metadata_workers: number;
    thumbnail_workers: number;
    sprites_workers: number;
}

export interface ProcessingConfig {
    max_frame_dimension_sm: number;
    max_frame_dimension_lg: number;
    frame_quality_sm: number;
    frame_quality_lg: number;
    frame_quality_sprites: number;
    sprites_concurrency: number;
}

export interface QueueStatus {
    metadata_queued: number;
    thumbnail_queued: number;
    sprites_queued: number;
    metadata_running: number;
    thumbnail_running: number;
    sprites_running: number;
}

export interface JobListResponse {
    data: JobHistory[];
    total: number;
    page: number;
    limit: number;
    active_count: number;
    active_jobs: JobHistory[];
    retention: string;
    pool_config: PoolConfig;
    queue_status: QueueStatus;
}

export interface TriggerConfig {
    id: number;
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'scan';
    trigger_type: 'on_import' | 'after_job' | 'manual' | 'scheduled';
    after_phase: string | null;
    cron_expression: string | null;
    updated_at: string;
}

export interface BulkJobRequest {
    phase: 'metadata' | 'thumbnail' | 'sprites';
    mode: 'missing' | 'all';
}

export interface BulkJobResponse {
    message: string;
    submitted: number;
    skipped: number;
    errors: number;
}

export interface DLQEntry {
    id: number;
    job_id: string;
    video_id: number;
    video_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites';
    original_error: string;
    failure_count: number;
    last_error: string;
    status: 'pending_review' | 'retrying' | 'abandoned';
    created_at: string;
    updated_at: string;
    abandoned_at?: string;
}

export interface DLQListResponse {
    data: DLQEntry[];
    total: number;
    page: number;
    limit: number;
    stats: {
        pending_review: number;
        retrying: number;
        abandoned: number;
        total: number;
    };
}

export interface RetryConfig {
    id: number;
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'scan';
    max_retries: number;
    initial_delay_seconds: number;
    max_delay_seconds: number;
    backoff_factor: number;
    updated_at: string;
}

// Job status types for header indicator
export interface JobStatusPhase {
    running: number;
    queued: number;
    pending: number;
}

export interface ActiveJobInfo {
    job_id: string;
    scene_id: number;
    scene_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites';
    started_at: string;
}

export interface JobStatusData {
    total_running: number;
    total_queued: number;
    total_pending: number;
    by_phase: Record<string, JobStatusPhase>;
    active_jobs: ActiveJobInfo[];
    more_count: number;
}
