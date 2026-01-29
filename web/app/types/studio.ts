// Lightweight studio for list views
export interface StudioListItem {
    id: number;
    uuid: string;
    name: string;
    short_name: string;
    logo: string;
    video_count: number;
}

// Full studio details for detail views
export interface Studio {
    id: number;
    uuid: string;
    created_at: string;
    updated_at: string;
    name: string;
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
    video_count?: number;
}

export interface StudioListResponse {
    data: StudioListItem[];
    total: number;
    page: number;
    limit: number;
}

export interface CreateStudioInput {
    name: string;
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
