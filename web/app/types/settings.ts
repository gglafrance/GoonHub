import type { HomepageConfig } from './homepage';
import type { ParsingRulesSettings } from './parsing-rules';
import type { DateFormat } from '~/utils/dateFormat';

export interface SortPreferences {
    actors: string;
    studios: string;
    markers: string;
    actor_scenes: string;
    studio_scenes: string;
}

export type MarkerView = 'grouped' | 'all';

// Scene list pages that can show their scenes as a grid of cards or as a table
export type SceneListPage = 'search' | 'actor' | 'studio' | 'playlist' | 'explorer';
export type SceneListViewMode = 'grid' | 'table';
export type SceneTableColumn =
    | 'thumbnail'
    | 'title'
    | 'duration'
    | 'resolution'
    | 'file_size'
    | 'rating'
    | 'views'
    | 'studio'
    | 'actors'
    | 'release_date'
    | 'added_at';

export interface SceneListView {
    // Missing page: grid
    pages: Partial<Record<SceneListPage, SceneListViewMode>>;
    // Visible table columns, in display order
    columns: SceneTableColumn[];
}
export type PaginationPosition = 'top' | 'bottom' | 'both';
// Card size of the scene grids (and actor/studio grids)
export type GridDensity = 'compact' | 'comfortable' | 'large';
export type { DateFormat };

export interface UserSettings {
    id: number;
    user_id: number;
    autoplay: boolean;
    default_volume: number;
    loop: boolean;
    ab_loop_controls: boolean;
    marker_loop: boolean;
    videos_per_page: number;
    default_sort_order: SortOrder;
    default_tag_sort: TagSort;
    marker_thumbnail_cycling: boolean;
    homepage_config: HomepageConfig;
    parsing_rules: ParsingRulesSettings;
    sort_preferences: SortPreferences;
    playlist_auto_advance: PlaylistAutoAdvance;
    playlist_countdown_seconds: number;
    show_page_size_selector: boolean;
    advanced_marker_filters: boolean;
    default_marker_view: MarkerView;
    pagination_position: PaginationPosition;
    date_format: DateFormat;
    scene_card_config: SceneCardConfig;
    scene_list_view: SceneListView;
    grid_density: GridDensity;
    infinite_scroll: boolean;
    max_items_per_page: number;
    created_at: string;
    updated_at: string;
}

export interface PlayerSettings {
    autoplay: boolean;
    default_volume: number;
    loop: boolean;
}

export interface AppSettings {
    videos_per_page: number;
    default_sort_order: SortOrder;
    marker_thumbnail_cycling: boolean;
}

export type SortOrder =
    | 'created_at_desc'
    | 'created_at_asc'
    | 'title_asc'
    | 'title_desc'
    | 'duration_asc'
    | 'duration_desc'
    | 'size_asc'
    | 'size_desc'
    | 'last_jizz_desc'
    | 'last_jizz_asc'
    | 'jizz_count_desc'
    | 'jizz_count_asc'
    | 'random';

export type TagSort = 'az' | 'za' | 'most' | 'least';

export type KeyboardLayout = 'qwerty' | 'azerty';

export type PlaylistAutoAdvance = 'instant' | 'countdown' | 'manual';

export interface TagSettings {
    default_tag_sort: TagSort;
}

export interface BadgeZone {
    items: string[];
    direction: 'vertical' | 'horizontal';
}

export interface BadgeZones {
    top_left: BadgeZone;
    top_right: BadgeZone;
    bottom_left: BadgeZone;
    bottom_right: BadgeZone;
}

export interface ContentRow {
    type: 'full' | 'split';
    field?: string;
    mode?: 'short' | 'long';
    left?: string;
    right?: string;
    left_mode?: 'short' | 'long';
    right_mode?: 'short' | 'long';
}

export interface SceneCardConfig {
    badges: BadgeZones;
    content_rows: ContentRow[];
}

export interface ChangePasswordRequest {
    current_password: string;
    new_password: string;
}

export interface ChangeUsernameRequest {
    username: string;
}
