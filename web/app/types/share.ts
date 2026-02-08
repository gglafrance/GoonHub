export interface ShareLink {
    id: number;
    token: string;
    scene_id: number;
    user_id: number;
    share_type: 'public' | 'auth_required';
    expires_at: string | null;
    view_count: number;
    created_at: string;
}

export interface ShareSceneData {
    id: number;
    title: string;
    description: string;
    duration: number;
    studio: string;
    tags: string[];
    actors: string[];
    created_at: string;
    release_date: string | null;
}

export interface ResolvedShareLink {
    share_link: ShareLink;
    scene: ShareSceneData;
}

export interface ShareLinksResponse {
    share_links: ShareLink[];
    share_base_url: string;
}
