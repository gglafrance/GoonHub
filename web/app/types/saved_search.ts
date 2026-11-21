export interface SavedSearchFilters {
    query?: string;
    match_type?: string;
    selected_tags?: string[];
    selected_actors?: string[];
    studio?: string;
    resolution?: string;
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
    has_source?: boolean;
    // normalized source sites (OR)
    source_sites?: string[];
    min_date?: string;
    max_date?: string;
    liked?: boolean;
    min_rating?: number;
    max_rating?: number;
    min_jizz_count?: number;
    max_jizz_count?: number;
    selected_marker_labels?: string[];
    sort?: string;
}

export interface SavedSearch {
    uuid: string;
    name: string;
    filters: SavedSearchFilters;
    created_at: string;
    updated_at: string;
}

export interface SavedSearchListResponse {
    data: SavedSearch[];
}

export interface CreateSavedSearchInput {
    name: string;
    filters: SavedSearchFilters;
}

export interface UpdateSavedSearchInput {
    name?: string;
    filters?: SavedSearchFilters;
}
