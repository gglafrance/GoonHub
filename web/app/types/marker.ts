export interface Marker {
    id: number;
    user_id: number;
    scene_id: number;
    timestamp: number;
    end_timestamp?: number | null;
    label: string;
    color: string;
    thumbnail_path: string;
    animated_thumbnail_path: string;
    created_at: string;
    updated_at: string;
    tags?: MarkerTagInfo[];
}

// Marker summary attached to scene cards (with the tags card field)
export interface SceneCardMarker {
    id: number;
    label: string;
    color: string;
    timestamp: number;
    end_timestamp?: number | null;
}

// Filters stored by a saved marker filter (marker pages advanced filters)
export interface SavedMarkerFilterValues {
    labels?: string[];
    marker_tags?: string[];
    tags?: string[];
    actors?: string[];
    studio?: string;
    ranges_only?: boolean;
    sort?: string;
    view?: 'grouped' | 'all';
}

export interface SavedMarkerFilter {
    id: number;
    uuid: string;
    name: string;
    filters: SavedMarkerFilterValues;
    created_at: string;
    updated_at: string;
}

export interface CreateMarkerRequest {
    timestamp: number;
    end_timestamp?: number;
    label?: string;
    color?: string;
}

export interface UpdateMarkerRequest {
    timestamp?: number;
    end_timestamp?: number;
    clear_end_timestamp?: boolean;
    label?: string;
    color?: string;
}

export interface MarkerLabelSuggestion {
    label: string;
    count: number;
}

export interface MarkerLabelGroup {
    label: string;
    count: number;
    thumbnail_marker_id: number;
    thumbnail_marker_ids: number[];
    // Thumbnail version (Unix ms) of each marker of thumbnail_marker_ids, by marker ID
    thumbnail_versions?: Record<number, number>;
}

export interface MarkerWithScene extends Marker {
    scene_title: string;
    tags?: MarkerTagInfo[];
}

// Tag info with metadata about source
export interface MarkerTagInfo {
    id: number;
    name: string;
    color: string;
    is_from_label: boolean;
}

// API Response Types
export interface MarkersResponse {
    markers: Marker[];
}

export interface LabelSuggestionsResponse {
    labels: MarkerLabelSuggestion[];
}

export interface PaginatedResponse<T> {
    data: T[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}

// Request types for tag operations
export interface SetLabelTagsRequest {
    tag_ids: number[];
}

export interface SetMarkerTagsRequest {
    tag_ids: number[];
}

// Response types for tag operations
export interface LabelTagsResponse {
    tags: import('~/types/tag').Tag[];
}

export interface MarkerTagsResponse {
    tags: MarkerTagInfo[];
}

// Deduplication of markers (admin maintenance, POST /admin/markers/deduplicate): per user, the
// markers with the same label (or none) whose starts are a couple of seconds apart are folded
// into one, which gets the tags of the others
export interface MarkerDedupMarker {
    id: number;
    timestamp: number;
    end_timestamp?: number;
    label: string;
}

export interface MarkerDedupMerge {
    scene_id: number;
    user_id: number;
    kept: MarkerDedupMarker;
    // Label of the kept marker after the merge
    label: string;
    removed: MarkerDedupMarker[];
    added_tag_ids: number[];
}

export interface MarkerDedupResult {
    dry_run: boolean;
    scenes_scanned: number;
    scenes: number;
    groups: number;
    removed: number;
    // At most 200 sets (truncated then); the counts cover all of them
    merges: MarkerDedupMerge[];
    truncated: boolean;
}

// Scenes to deduplicate: some of them, or every scene
export type MarkerDedupScope = { scene_ids: number[] } | { all: true };
