// Profile pages API shapes (contract: docs/PROFILES.md).
import type { SceneListItem } from '~/types/scene';
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';
import type { PrivacySettings, SocialProfile, SocialUser } from '~/types/social';

export type ProfileBlockType =
    | 'favorite_scenes'
    | 'favorite_markers'
    | 'recent_watches'
    | 'most_watched'
    | 'recent_markers'
    | 'liked_scenes'
    | 'stats'
    | 'activity_calendar'
    | 'playlists'
    | 'friends';

export type ProfileBlockConfig = Record<string, number | string>;

/** A scene of a block with its figure (watched or liked at, views). */
export interface ProfileScene {
    scene: SceneListItem;
    at?: string;
    views?: number;
}

/** A block as the viewer gets it. A locked block has no data. */
export interface ProfileBlock<TWidget = unknown> {
    type: ProfileBlockType | string;
    label: string;
    privacy_item: string;
    config: ProfileBlockConfig;
    locked: boolean;
    failed?: boolean;
    empty: boolean;
    scenes?: ProfileScene[];
    markers?: WidgetMarker[];
    widget?: TWidget;
    total?: number;
}

export interface ProfileBanner {
    type: 'scene' | 'upload';
    url: string;
    url_md: string;
    scene_id?: number;
}

/** GET /api/v1/profiles/users/:username */
export interface Profile extends SocialProfile {
    bio: string;
    banner: ProfileBanner | null;
    blocks: ProfileBlock[];
}

export interface ProfileBlockOption {
    key: string;
    label: string;
    kind: 'int' | 'choice';
    min?: number;
    max?: number;
    default: number | string;
    choices?: { value: string; label: string }[];
}

export interface ProfileBlockTypeInfo {
    type: string;
    label: string;
    description: string;
    privacy_item: string;
    viewer_permission?: string;
    options: ProfileBlockOption[];
}

/** A block of the stored layout. */
export interface ProfileLayoutBlock {
    type: string;
    visible: boolean;
    locked_placeholder: boolean;
    config: ProfileBlockConfig;
}

export interface ProfileLimits {
    max_showcase_items: number;
    bio_max_length: number;
    display_name_max_length: number;
    max_upload_bytes: number;
}

/** GET /api/v1/profiles/me */
export interface ProfileEdit {
    user: SocialUser;
    display_name: string;
    bio: string;
    avatar_version: number;
    banner: { type: 'none' | 'scene' | 'upload'; scene_id?: number; version: number; url?: string };
    layout: ProfileLayoutBlock[];
    favorite_scenes: SceneListItem[];
    favorite_markers: WidgetMarker[];
    block_types: ProfileBlockTypeInfo[];
    privacy: PrivacySettings;
    limits: ProfileLimits;
}

/** An item of a showcase being edited. */
export interface ShowcaseItem {
    id: number;
    title: string;
    thumb: string | null;
}

/** Crop rectangle of an image upload, in source pixels. */
export interface ImageCrop {
    x: number;
    y: number;
    w: number;
    h: number;
}

// Payloads of the widget blocks (internal/core/profile_block_*.go)

export interface ProfileTopEntry {
    id: number;
    uuid?: string;
    name: string;
    views: number;
}

export interface ProfileStatsPayload {
    period: string;
    watch_time: number;
    views: number;
    scenes_watched: number;
    jizz: number;
    liked: number;
    markers: number;
    rated: number;
    avg_rating: number | null;
    top_tags: ProfileTopEntry[];
    top_actors: ProfileTopEntry[];
    top_studios: ProfileTopEntry[];
}

export interface ProfilePlaylistsPayload {
    items: {
        uuid: string;
        name: string;
        scene_count: number;
        cover_scene_id?: number;
        updated_at: string;
    }[];
}

export interface ProfileFriendsPayload {
    items: {
        id: number;
        username: string;
        display_name: string;
        avatar_version: number;
        since: string;
    }[];
}
