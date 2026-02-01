export interface Marker {
    id: number;
    user_id: number;
    scene_id: number;
    timestamp: number;
    label: string;
    color: string;
    thumbnail_path: string;
    created_at: string;
    updated_at: string;
    tags?: MarkerTagInfo[];
}

export interface CreateMarkerRequest {
    timestamp: number;
    label?: string;
    color?: string;
}

export interface UpdateMarkerRequest {
    timestamp?: number;
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
