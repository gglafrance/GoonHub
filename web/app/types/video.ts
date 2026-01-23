export interface Video {
    id: number;
    title: string;
    original_filename: string;
    size: number;
    view_count: number;
    created_at: string;
    duration: number;
    width?: number;
    height?: number;
    thumbnail_path?: string;
    sprite_sheet_path?: string;
    vtt_path?: string;
    sprite_sheet_count?: number;
    thumbnail_width?: number;
    thumbnail_height?: number;
    processing_status?: string;
    processing_error?: string;
}

export interface VideoListResponse {
    data: Video[];
    total: number;
    page: number;
    limit: number;
}
