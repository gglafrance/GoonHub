import type { SceneListItem } from './scene';

export interface PlaylistOwner {
    id: number;
    username: string;
}

export interface PlaylistTag {
    id: number;
    name: string;
    color: string;
}

export interface PlaylistThumbnailScene {
    id: number;
    thumbnail_path: string;
}

export interface PlaylistListItem {
    uuid: string;
    name: string;
    description: string | null;
    visibility: string;
    scene_count: number;
    total_duration: number;
    owner: PlaylistOwner;
    tags: PlaylistTag[];
    thumbnail_scenes: PlaylistThumbnailScene[];
    is_liked: boolean;
    like_count: number;
    created_at: string;
    updated_at: string;
}

export interface PlaylistSceneEntry {
    position: number;
    scene: SceneListItem;
    added_at: string;
}

export interface PlaylistResume {
    scene_id: number | null;
    position_s: number;
}

export interface PlaylistDetail extends PlaylistListItem {
    scenes: PlaylistSceneEntry[];
    resume: PlaylistResume | null;
}

export interface PlaylistListResponse {
    data: PlaylistListItem[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}

export interface CreatePlaylistInput {
    name: string;
    description?: string;
    visibility?: string;
    tag_ids?: number[];
    scene_ids?: number[];
}

export interface UpdatePlaylistInput {
    name?: string;
    description?: string;
    visibility?: string;
}

export type PlaylistSortOption =
    | 'created_at_desc'
    | 'created_at_asc'
    | 'name_asc'
    | 'name_desc'
    | 'scene_count_desc'
    | 'updated_at_desc';

export type PlaylistOwnerFilter = 'me' | 'all';
export type PlaylistVisibilityFilter = '' | 'public' | 'private' | 'unlisted';

export const PLAYLIST_SORT_OPTIONS: { value: PlaylistSortOption; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'name_asc', label: 'Name A-Z' },
    { value: 'name_desc', label: 'Name Z-A' },
    { value: 'scene_count_desc', label: 'Most Scenes' },
    { value: 'updated_at_desc', label: 'Recently Updated' },
];
