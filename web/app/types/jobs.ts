export interface JobHistory {
    id: number;
    job_id: string;
    scene_id: number;
    scene_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint';
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
    animated_thumbnails_workers: number;
    fingerprint_workers: number;
    duplication_enabled: boolean;
}

export interface ProcessingConfig {
    max_frame_dimension_sm: number;
    max_frame_dimension_lg: number;
    frame_quality_sm: number;
    frame_quality_lg: number;
    frame_quality_sprites: number;
    sprites_concurrency: number;
    marker_thumbnail_type: string;
    marker_animated_duration: number;
    scene_preview_enabled: boolean;
    scene_preview_segments: number;
    scene_preview_segment_duration: number;
    marker_preview_crf: number;
    scene_preview_crf: number;
}

export interface QueueStatus {
    metadata_queued: number;
    thumbnail_queued: number;
    sprites_queued: number;
    animated_thumbnails_queued: number;
    fingerprint_queued: number;
    metadata_running: number;
    thumbnail_running: number;
    sprites_running: number;
    animated_thumbnails_running: number;
    fingerprint_running: number;
    metadata_pending: number;
    thumbnail_pending: number;
    sprites_pending: number;
    animated_thumbnails_pending: number;
    fingerprint_pending: number;
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
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint' | 'scan';
    trigger_type: 'on_import' | 'after_job' | 'manual' | 'scheduled';
    after_phase: string | null;
    cron_expression: string | null;
    updated_at: string;
}

export interface BulkJobRequest {
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint';
    mode: 'missing' | 'all';
    force_target?: 'markers' | 'previews' | 'both';
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
    scene_id: number;
    scene_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint';
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
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint' | 'scan';
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
    failed: number;
}

export interface ActiveJobInfo {
    job_id: string;
    scene_id: number;
    scene_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint';
    started_at: string;
}

export interface JobStatusData {
    total_running: number;
    total_queued: number;
    total_pending: number;
    total_failed: number;
    by_phase: Record<string, JobStatusPhase>;
    active_jobs: ActiveJobInfo[];
    more_count: number;
}
