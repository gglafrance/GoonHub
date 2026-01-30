// VideoListItem is a lightweight video representation for list/grid displays.
// Used by: homepage sections, search results, actor/studio videos, explorer, history
export interface VideoListItem {
    id: number;
    title: string;
    duration: number;
    size: number;
    thumbnail_path: string;
    processing_status: string;
    created_at: string;
    updated_at: string;
}

// Video is the full video representation with all metadata.
// Used by: video player page, video details editor
export interface Video extends VideoListItem {
    original_filename: string;
    view_count: number;
    width?: number;
    height?: number;
    sprite_sheet_path?: string;
    vtt_path?: string;
    sprite_sheet_count?: number;
    thumbnail_width?: number;
    thumbnail_height?: number;
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
    release_date?: string;
    porndb_scene_id?: string;
}

export interface VideoListResponse {
    data: VideoListItem[];
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
