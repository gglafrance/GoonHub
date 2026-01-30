export interface SavedSearchFilters {
    query?: string;
    match_type?: string;
    selected_tags?: string[];
    selected_actors?: string[];
    studio?: string;
    resolution?: string;
    min_duration?: number;
    max_duration?: number;
    min_date?: string;
    max_date?: string;
    liked?: boolean;
    min_rating?: number;
    max_rating?: number;
    min_jizz_count?: number;
    max_jizz_count?: number;
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
