export interface Video {
    id: number;
    title: string;
    original_filename: string;
    size: number;
    view_count: number;
    created_at: string;
    duration: number;
}

export interface VideoListResponse {
    data: Video[];
    total: number;
    page: number;
    limit: number;
}
