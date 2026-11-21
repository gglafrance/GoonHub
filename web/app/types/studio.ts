// Lightweight studio for list views
export interface StudioListItem {
    id: number;
    uuid: string;
    name: string;
    aliases: string[];
    short_name: string;
    logo: string;
    scene_count: number;
    // Alias that matched the search query when the name does not
    matched_alias?: string;
}

// Full studio details for detail views
export interface Studio {
    id: number;
    uuid: string;
    created_at: string;
    updated_at: string;
    name: string;
    aliases?: string[];
    short_name?: string;
    url?: string;
    description?: string;
    rating?: number;
    logo?: string;
    favicon?: string;
    poster?: string;
    porndb_id?: string;
    parent_id?: number;
    network_id?: number;
    scene_count?: number;
}

export interface StudioListResponse {
    data: StudioListItem[];
    total: number;
    page: number;
    limit: number;
}

export interface CreateStudioInput {
    name: string;
    aliases?: string[];
    short_name?: string;
    url?: string;
    description?: string;
    rating?: number;
    logo?: string;
    favicon?: string;
    poster?: string;
    porndb_id?: string;
    parent_id?: number;
    network_id?: number;
}

export interface UpdateStudioInput {
    name?: string;
    aliases?: string[];
    short_name?: string;
    url?: string;
    description?: string;
    rating?: number;
    logo?: string;
    favicon?: string;
    poster?: string;
    porndb_id?: string;
    parent_id?: number;
    network_id?: number;
}

export interface StudioInteractions {
    rating: number;
    liked: boolean;
}

// Answer of GET /studios/resolve: exact name-or-alias match
export interface StudioResolution {
    data: Studio;
    matched: 'name' | 'alias';
    matched_alias: string;
    candidates: number;
}

// What merging a source studio into a target studio moves
export interface StudioMergePreview {
    target: Studio;
    source: Studio;
    scenes: number;
    child_studios: number;
    ratings: number;
    likes: number;
    aliases_added: string[];
    logo_from_source: boolean;
}
