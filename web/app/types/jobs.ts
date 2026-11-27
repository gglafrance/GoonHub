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

export type SchedulablePhase =
    | 'metadata'
    | 'thumbnail'
    | 'sprites'
    | 'animated_thumbnails'
    | 'fingerprint';

export interface PhaseScheduleConfig {
    phase: SchedulablePhase;
    priority: number;
    window_enabled: boolean;
    window_start: string;
    window_end: string;
    quiet_workers: number;
}

export interface JobScheduleConfig {
    timezone: string;
    quiet_hours_start: string;
    quiet_hours_end: string;
    max_concurrent_jobs: number;
    new_scenes_first: boolean;
    new_scenes_window_hours: number;
    phases: PhaseScheduleConfig[];
}

export interface PhaseScheduleState {
    phase: SchedulablePhase;
    priority: number;
    state: 'running' | 'paused';
    next_change?: string;
    workers: number;
    configured_workers: number;
    quiet_workers: number;
    worker_mode: 'day' | 'quiet';
    pending: number;
    queued: number;
    running: number;
}

export interface JobScheduleStatus {
    now: string;
    timezone: string;
    quiet_hours: boolean;
    quiet_hours_next_change: string;
    max_concurrent_jobs: number;
    running_jobs: number;
    new_scene_since?: string;
    phases: PhaseScheduleState[];
}

export interface JobScheduleResponse {
    config: JobScheduleConfig;
    status: JobScheduleStatus;
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
    // Failing scenes (failures in a row on an unchanged file) left out of a "missing" run
    skipped_failing: number;
    errors: number;
}

// A (scene, phase) that failed processing.failing_threshold times in a row on an unchanged
// file: the "missing" runs and the automatic retries skip it (Settings > Jobs > Failing).
export interface FailingScene {
    id: number;
    scene_id: number;
    scene_title: string;
    stored_path: string;
    phase: string;
    failures: number;
    last_error: string;
    last_failed_at: string;
    file_size: number | null;
}

export interface FailingSceneList {
    data: FailingScene[];
    total: number;
    page: number;
    limit: number;
    // Failing (scene, phase) per phase, error filter applied, phase filter ignored
    totals: Record<string, number>;
    threshold: number;
}

export interface FailingActionResult {
    retried: number;
    cleared: number;
    errors: number;
}

export type FailingAction = 'retry' | 'clear';

// Body of POST /admin/jobs/failing/bulk: the given IDs, or every failing match of the filters
export interface FailingBulkRequest {
    action: FailingAction;
    ids?: number[];
    all?: boolean;
    phase?: string;
    q?: string;
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

export type BulkCancelPhase =
    | 'all'
    | 'metadata'
    | 'thumbnail'
    | 'sprites'
    | 'animated_thumbnails'
    | 'fingerprint';

export interface BulkCancelResponse {
    message: string;
    phases: string[];
    pending_cancelled: number;
    queued_skipped: number;
    running_cancelled: number;
    include_running: boolean;
}

export interface HistoryPurgeSettings {
    completed_retention: string;
    failed_retention: string;
    completed_enabled: boolean;
    failed_enabled: boolean;
    interval: string;
    batch_size: number;
}

export interface HistoryPurgePreview {
    completed: number;
    cancelled: number;
    failed: number;
    timed_out: number;
    total: number;
    completed_before?: string;
    failed_before?: string;
}

export interface HistoryPurgeResult {
    trigger: 'scheduled' | 'manual';
    completed_deleted: number;
    failed_deleted: number;
    total: number;
    batches: number;
    started_at: string;
    duration_ms: number;
    error?: string;
}

export interface HistoryPurgeInfo {
    settings: HistoryPurgeSettings;
    preview: HistoryPurgePreview;
    last_run: HistoryPurgeResult | null;
}

export interface HistoryPurgeResponse {
    message: string;
    result: HistoryPurgeResult;
}
