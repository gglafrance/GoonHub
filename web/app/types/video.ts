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
    file_created_at?: string;
    description?: string;
    studio?: string;
    tags?: string[];
    actors?: string[];
    cover_image_path?: string;
    file_hash?: string;
    frame_rate?: number;
    bit_rate?: number;
    video_codec?: string;
    audio_codec?: string;
}

export interface VideoListResponse {
    data: Video[];
    total: number;
    page: number;
    limit: number;
}

export interface VideoSearchParams {
    q?: string;
    tags?: string;
    actors?: string;
    studio?: string;
    min_duration?: number;
    max_duration?: number;
    min_date?: string;
    max_date?: string;
    resolution?: string;
    sort?: string;
    page?: number;
    limit?: number;
    liked?: boolean;
    min_rating?: number;
    max_rating?: number;
    min_jizz_count?: number;
    max_jizz_count?: number;
}

export interface TagOption {
    id: number;
    name: string;
    color: string;
    video_count: number;
}

export interface VideoFilterOptions {
    studios: string[];
    actors: string[];
    tags: TagOption[];
}
