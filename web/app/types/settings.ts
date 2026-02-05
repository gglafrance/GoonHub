import type { HomepageConfig } from './homepage';
import type { ParsingRulesSettings } from './parsing-rules';

export interface SortPreferences {
    actors: string;
    studios: string;
    markers: string;
    actor_scenes: string;
    studio_scenes: string;
}

export interface UserSettings {
    id: number;
    user_id: number;
    autoplay: boolean;
    default_volume: number;
    loop: boolean;
    videos_per_page: number;
    default_sort_order: SortOrder;
    default_tag_sort: TagSort;
    marker_thumbnail_cycling: boolean;
    homepage_config: HomepageConfig;
    parsing_rules: ParsingRulesSettings;
    sort_preferences: SortPreferences;
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
    | 'random';

export type TagSort = 'az' | 'za' | 'most' | 'least';

export type KeyboardLayout = 'qwerty' | 'azerty';

export interface TagSettings {
    default_tag_sort: TagSort;
}

export interface ChangePasswordRequest {
    current_password: string;
    new_password: string;
}

export interface ChangeUsernameRequest {
    username: string;
}
