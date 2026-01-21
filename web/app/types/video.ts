export interface Video {
    id: number;
    title: string;
    original_filename: string;
    size: number;
    view_count: number;
    created_at: string;
    duration: number;
    thumbnail_path?: string;
    frame_paths?: string;
    frame_count?: number;
    frame_interval?: number;
    processing_status?: string;
    processing_error?: string;
}

export interface VideoListResponse {
    data: Video[];
    total: number;
    page: number;
    limit: number;
}
