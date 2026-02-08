// SceneListItem is a lightweight scene representation for list/grid displays.
// Used by: homepage sections, search results, actor/studio scenes, explorer, history
export interface SceneListItem {
    id: number;
    title: string;
    duration: number;
    size: number;
    thumbnail_path: string;
    preview_video_path: string;
    processing_status: string;
    is_corrupted: boolean;
    created_at: string;
    updated_at: string;
    // Optional fields included via card_fields
    view_count?: number;
    width?: number;
    height?: number;
    frame_rate?: number;
    description?: string;
    studio?: string;
    tags?: string[];
    actors?: string[];
}

// Scene is the full scene representation with all metadata.
// Used by: scene player page, scene details editor
export interface Scene extends SceneListItem {
    original_filename: string;
    stored_path: string;
    storage_path_id?: number;
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
    origin?: string;
    type?: string;
}

export interface SceneListResponse {
    data: SceneListItem[];
    total: number;
    page: number;
    limit: number;
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizz_counts?: Record<string, number>;
    seed?: number;
}

export interface SceneSearchParams {
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
    origin?: string;
    type?: string;
}

export interface TagOption {
    id: number;
    name: string;
    color: string;
    scene_count: number;
}

export interface MarkerLabelOption {
    label: string;
    count: number;
}

export interface SceneFilterOptions {
    studios: string[];
    actors: string[];
    tags: TagOption[];
    marker_labels: MarkerLabelOption[];
    origins: string[];
    types: string[];
}
