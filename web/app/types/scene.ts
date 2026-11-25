// SceneListItem is a lightweight scene representation for list/grid displays.
// Used by: homepage sections, search results, actor/studio scenes, explorer, history
export interface SceneListItem {
    id: number;
    title: string;
    duration: number;
    // Omitted by the API without files:view_size
    size?: number;
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
    // Omitted without files:view_technical
    frame_rate?: number;
    description?: string;
    studio?: string;
    tags?: string[];
    actors?: string[];
    markers?: import('~/types/marker').SceneCardMarker[];
    // Included via the release_date card field (scene tables)
    release_date?: string;
}

// Scene is the full scene representation with all metadata.
// Used by: scene player page, scene details editor
export interface Scene extends SceneListItem {
    // Omitted by the API without files:view_paths
    original_filename?: string;
    stored_path?: string;
    storage_path_id?: number;
    view_count: number;
    width?: number;
    height?: number;
    sprite_sheet_path?: string;
    vtt_path?: string;
    sprite_sheet_count?: number;
    thumbnail_width?: number;
    thumbnail_height?: number;
    // processing_error, file_hash, fingerprint_*, frame_rate, bit_rate and the codecs are
    // omitted without files:view_technical
    processing_error?: string;
    file_created_at?: string;
    description?: string;
    // the web page the scene comes from ('' or absent: unknown)
    source_url?: string;
    // site of the source URL, normalized by the server ("pornhub.com"; '' without a source)
    source_site?: string;
    studio?: string;
    tags?: string[];
    actors?: string[];
    cover_image_path?: string;
    file_hash?: string;
    fingerprint_type?: string;
    fingerprint_at?: string;
    frame_rate?: number;
    bit_rate?: number;
    video_codec?: string;
    audio_codec?: string;
    release_date?: string;
    porndb_scene_id?: string;
    origin?: string;
    type?: string;
    // how the player can play the file (scene detail endpoint only)
    playback?: ScenePlayback;
}

// direct: the browser plays the original; remux: the original streams are copied into an MP4
// on the fly; transcode: the video is re-encoded on the fly (H.264 + AAC)
export type ScenePlaybackMode = 'direct' | 'remux' | 'transcode';

export interface ScenePlayback {
    mode: ScenePlaybackMode;
    // what browsers can't play ('' or absent for direct)
    reason?: string;
    // false: the admin turned on-the-fly transcoding off, only direct scenes play
    transcode_enabled: boolean;
}

// Why the player can't play a scene: disabled (conversion turned off), limit (all conversion
// slots in use), failed (conversion error)
export interface PlaybackNotice {
    kind: 'disabled' | 'limit' | 'failed';
    reason?: string;
}

export interface SceneListResponse {
    data: SceneListItem[];
    total: number;
    page: number;
    limit: number;
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizz_counts?: Record<string, number>;
    // scenes the user watched to the end (card_fields "watched")
    watched?: Record<string, boolean>;
    seed?: number;
}

export interface SceneSearchParams {
    q?: string;
    tags?: string;
    actors?: string;
    studio?: string;
    min_duration?: number;
    max_duration?: number;
    // File size bounds in megabytes
    min_size?: number;
    max_size?: number;
    // Library status filters (absent = any)
    watched?: boolean;
    rated?: boolean;
    has_markers?: boolean;
    has_actors?: boolean;
    has_tags?: boolean;
    has_studio?: boolean;
    // comma-separated source sites (OR), and source presence
    source_site?: string;
    has_source?: boolean;
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

// A file with the same content as the scene's file at another path, recognised by the
// library scan instead of being added as a new scene (GET /scenes/:id/copies)
export interface SceneCopy {
    id: number;
    scene_id: number;
    stored_path: string;
    storage_path_id: number | null;
    size: number;
    oshash: string;
    created_at: string;
    last_seen_at: string;
}

export interface SceneCopiesResponse {
    copies: SceneCopy[];
}

// A site scenes come from, with its scene count (GET /scenes/source-sites)
export interface SourceSiteOption {
    site: string;
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
