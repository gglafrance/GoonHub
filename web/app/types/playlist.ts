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

/** What a playlist holds, chosen at creation (docs/MARKER_PLAYLISTS.md). */
export type PlaylistKind = 'scenes' | 'markers';

/** A marker thumbnail of a marker playlist card: /marker-thumbnails/:id?v=version. */
export interface PlaylistThumbnailMarker {
    id: number;
    version: number;
}

export interface PlaylistListItem {
    uuid: string;
    name: string;
    description: string | null;
    visibility: string;
    kind: PlaylistKind;
    /** Scenes of a scene playlist; scenes the markers come from in a marker playlist */
    scene_count: number;
    marker_count: number;
    /** Seconds: scene durations, or marker play lengths in a marker playlist */
    total_duration: number;
    owner: PlaylistOwner;
    tags: PlaylistTag[];
    thumbnail_scenes: PlaylistThumbnailScene[];
    thumbnail_markers: PlaylistThumbnailMarker[];
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

/** A marker of a marker playlist; it plays from `start` to `end` (seconds of its scene). */
export interface PlaylistMarkerEntry {
    position: number;
    added_at: string;
    marker: {
        id: number;
        user_id: number;
        scene_id: number;
        timestamp: number;
        end_timestamp: number | null;
        label: string;
        color: string;
        has_thumbnail: boolean;
        thumbnail_version: number;
    };
    scene: {
        id: number;
        title: string;
        duration: number;
    };
    start: number;
    end: number;
}

export interface PlaylistResume {
    scene_id: number | null;
    /** Last marker played, in a marker playlist */
    marker_id?: number | null;
    position_s: number;
}

export interface PlaylistDetail extends PlaylistListItem {
    /** Scene playlists (empty for a marker playlist) */
    scenes: PlaylistSceneEntry[];
    /** Marker playlists (empty for a scene playlist) */
    markers: PlaylistMarkerEntry[];
    /** Play length of a marker without an end, in seconds */
    marker_default_duration: number;
    resume: PlaylistResume | null;
}

/** Markers of a marker listing to add at once: a label page, or the filtered list of all markers. */
export interface MarkerPlaylistFilter {
    label?: string;
    labels?: string;
    tags?: string;
    actors?: string;
    studio?: string;
    marker_tags?: string;
    ranges_only?: boolean;
    sort?: string;
    seed?: number;
}

export interface AddPlaylistMarkersResult {
    added: number;
    skipped: number;
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
    kind?: PlaylistKind;
    tag_ids?: number[];
    scene_ids?: number[];
    marker_ids?: number[];
}

export interface CreatePlaylistResult {
    uuid: string;
    name: string;
    kind: PlaylistKind;
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
export type PlaylistKindFilter = '' | PlaylistKind;
export type PlaylistVisibilityFilter = '' | 'public' | 'private' | 'unlisted';

export const PLAYLIST_SORT_OPTIONS: { value: PlaylistSortOption; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'name_asc', label: 'Name A-Z' },
    { value: 'name_desc', label: 'Name Z-A' },
    { value: 'scene_count_desc', label: 'Most Items' },
    { value: 'updated_at_desc', label: 'Recently Updated' },
];
