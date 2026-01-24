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

export interface JobListResponse {
    data: JobHistory[];
    total: number;
    page: number;
    limit: number;
    active_count: number;
    retention: string;
    pool_config: PoolConfig;
}
