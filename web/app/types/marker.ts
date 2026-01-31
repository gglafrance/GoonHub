export interface Marker {
    id: number;
    user_id: number;
    video_id: number;
    timestamp: number;
    label: string;
    color: string;
    thumbnail_path: string;
    created_at: string;
    updated_at: string;
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

export interface MarkerWithVideo extends Marker {
    video_title: string;
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
