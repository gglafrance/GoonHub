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
