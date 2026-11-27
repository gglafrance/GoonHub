// Profile pages API shapes (contract: docs/PROFILES.md).
import type { SceneListItem } from '~/types/scene';
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';
import type { Audience, PrivacySettings, SocialProfile, SocialUser } from '~/types/social';
import type { SavedSearchFilters } from '~/types/saved_search';
import type { CardSidecarResponse } from '~/utils/cardSidecars';

export type ProfileBlockType =
    | 'favorite_scenes'
    | 'favorite_markers'
    | 'recent_watches'
    | 'most_watched'
    | 'recent_markers'
    | 'liked_scenes'
    | 'stats'
    | 'ratings'
    | 'jizz'
    | 'activity_calendar'
    | 'playlists'
    | 'friends'
    | 'custom_list';

export type ProfileBlockConfig = Record<string, number | string>;

/**
 * A scene of a block or list with its figure: watched, liked, rated or last jizzed at, views,
 * the owner's rating (ratings list) or O count (jizz block and list).
 */
export interface ProfileScene {
    scene: SceneListItem;
    at?: string;
    views?: number;
    rating?: number;
    jizz_count?: number;
    /** The owner's figures of a custom list scene (the kinds the list shows) */
    figures?: ProfileSceneFigures;
}

/** The figure shown under the scene cards of a block or list. */
export type ProfileSceneFigureKind = 'at' | 'views' | 'rating' | 'jizz' | 'none';

/** A figure of a custom list: O count (and last O), rating, like date, views (and last watch). */
export type ListFigureKind = 'jizz' | 'rating' | 'liked' | 'views';

/** The figures setting of a custom list: from the sort and filters, none, or one kind. */
export type ListFiguresSetting = 'auto' | 'none' | ListFigureKind;

/** The owner's figures of a custom list scene; absent: not shown, or no such interaction. */
export interface ProfileSceneFigures {
    jizz_count?: number;
    last_jizz_at?: string;
    rating?: number;
    liked_at?: string;
    views?: number;
    last_watched_at?: string;
}

/** A block as the viewer gets it. A locked block has no data. */
export interface ProfileBlock<TWidget = unknown> {
    id?: string;
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

/** The full lists of a profile the viewer may open (the tabs). */
export interface ProfileListsVisibility {
    liked: boolean;
    ratings: boolean;
    jizz: boolean;
}

/** GET /api/v1/profiles/users/:username */
export interface Profile extends SocialProfile {
    bio: string;
    banner: ProfileBanner | null;
    blocks: ProfileBlock[];
    lists: ProfileListsVisibility;
    /** The tabs the viewer may open, the served tab and its sections */
    tabs: ProfileTabInfo[];
    tab: string;
    tab_missing?: boolean;
    sections: ProfileSectionView[];
}

/** Kinds of tabs: the profile, the built-in lists and the owner's custom tabs. */
export type ProfileTabKind = 'profile' | 'likes' | 'ratings' | 'jizz' | 'custom';

/** A tab as a viewer lists it. */
export interface ProfileTabInfo {
    id: string;
    kind: ProfileTabKind;
    title: string;
    icon?: string;
    slug?: string;
}

/** A section of the served tab: its column template and the blocks of each column. */
export interface ProfileSectionView {
    id: string;
    template: string;
    columns: ProfileBlock[][];
}

/** A section as the owner edits it. */
export interface ProfileSectionConfig {
    id: string;
    template: string;
    columns: ProfileLayoutBlock[][];
}

/** A tab as the owner edits it (sections for the profile and custom tabs). */
export interface ProfileTabConfig {
    id: string;
    kind: ProfileTabKind;
    title?: string;
    icon?: string;
    slug?: string;
    visible: boolean;
    sections?: ProfileSectionConfig[];
}

/** A column template of the sections (widths in twelfths). */
export interface ProfileSectionTemplate {
    id: string;
    label: string;
    widths: number[];
}

/** A privacy item a custom list depends on, with the owner's audience. */
export interface CustomListDependency {
    key: string;
    label: string;
    audience: Audience;
}

/** A custom list as its owner edits it. */
export interface CustomList {
    id: number;
    title: string;
    filters: SavedSearchFilters;
    saved_search_uuid: string | null;
    follows_saved_search: boolean;
    audience: Audience;
    dependencies: CustomListDependency[];
    needs_size: boolean;
    /** Which owner figures the cards show */
    figures: ListFiguresSetting;
    /** What "auto" shows for the current criteria, main first */
    auto_figures: ListFigureKind[];
    updated_at: string;
}

/** Body of POST /profiles/me/lists and PUT /profiles/me/lists/:id. */
export interface CustomListInput {
    title: string;
    filters: SavedSearchFilters;
    saved_search_uuid?: string;
    audience: Audience;
    figures?: ListFiguresSetting;
}

/** Payload of a custom_list block. */
export interface ProfileCustomListPayload {
    list_id: number;
    title: string;
    display: 'grid' | 'row';
    total: number;
    /** The owner's figures under the cards, main first (those the viewer may see) */
    figures?: ListFigureKind[];
}

/** A full list of a profile (route segment and API path). */
export type ProfileListKind = 'likes' | 'ratings' | 'jizz';

/**
 * GET /api/v1/profiles/users/:username/{likes,ratings,jizz} and /lists/:id (a custom list:
 * `list`, no `sort`).
 */
export interface ProfileListResponse extends CardSidecarResponse {
    data: ProfileScene[];
    pagination: { page: number; limit: number; total_items: number; total_pages: number };
    sort?: string;
    list?: { id: number; title: string };
    /** A custom list: the owner's figures on the items, main first */
    figures?: ListFigureKind[];
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
    /** Placed as many times as wanted (custom lists) */
    multi?: boolean;
}

/** A block of the stored layout. */
export interface ProfileLayoutBlock {
    id?: string;
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
    max_tabs: number;
    max_sections: number;
    max_blocks_per_tab: number;
    max_lists: number;
}

/** GET /api/v1/profiles/me */
export interface ProfileEdit {
    user: SocialUser;
    display_name: string;
    bio: string;
    avatar_version: number;
    banner: { type: 'none' | 'scene' | 'upload'; scene_id?: number; version: number; url?: string };
    layout: ProfileLayoutBlock[];
    tabs: ProfileTabConfig[];
    lists: CustomList[];
    section_templates: ProfileSectionTemplate[];
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

/** A bar of the ratings chart. */
export interface ProfileRatingBucket {
    rating: number;
    count: number;
}

/** Ratings block payload and GET /profiles/users/:username/ratings/histogram */
export interface ProfileRatingsPayload {
    buckets: ProfileRatingBucket[];
    total: number;
    average: number | null;
}

/** Jizz block payload: the O total and the number of videos (still in the library). */
export interface ProfileJizzPayload {
    total: number;
    scenes: number;
}
