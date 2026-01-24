export interface JobHistory {
    id: number;
    job_id: string;
    video_id: number;
    video_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites';
    status: 'running' | 'completed' | 'failed';
    error_message?: string;
    started_at: string;
    completed_at?: string;
    created_at: string;
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
