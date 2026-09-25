import type { SceneListItem } from '~/types/scene';

export type ShortsSort = 'newest' | 'random';
export type ClipsFilter = 'all' | 'only' | 'hide';

// The search page's Shorts filter. Where it starts is an admin setting.
export type ShortsSearchFilter = 'all' | 'hide' | 'only' | 'hide_clips' | 'only_clips';

export const DEFAULT_SHORTS_SEARCH: ShortsSearchFilter = 'hide';

export const SHORTS_SEARCH_LABELS: Record<ShortsSearchFilter, string> = {
    all: 'Show all',
    hide: 'Hide all shorts',
    only: 'Only shorts',
    hide_clips: 'Hide created clips',
    only_clips: 'Only created clips',
};

export const SHORTS_SEARCH_OPTIONS = Object.entries(SHORTS_SEARCH_LABELS).map(([value, label]) => ({
    value: value as ShortsSearchFilter,
    label,
}));

export const isShortsSearchFilter = (v: unknown): v is ShortsSearchFilter =>
    typeof v === 'string' && v in SHORTS_SEARCH_LABELS;

// A scene in the Shorts feed or a scene's Shorts tab.
export interface ShortItem extends SceneListItem {
    width: number;
    height: number;
    view_count: number;
    source_scene_id: number | null;
    source_start: number | null;
    source_end: number | null;
}

export interface ShortsFeedResponse {
    data: ShortItem[];
    total: number;
    page: number;
    limit: number;
    seed?: number;
    max_duration: number;
    likes?: Record<string, boolean>;
    ratings?: Record<string, number>;
    jizz_counts?: Record<string, number>;
}

export interface ShortsConfig {
    max_duration: number;
    // The user has scenes:upload.
    can_upload: boolean;
    // The shorts folder resolves and is writable.
    can_create: boolean;
    // What the search page's Shorts filter starts on.
    search_default: ShortsSearchFilter;
}

// A short still being encoded.
export interface ShortTask {
    task_id: string;
    source_scene_id: number;
    start: number;
    end: number;
    title: string;
    percent: number;
    status: 'queued' | 'running' | 'failed';
    error?: string;
}

export interface SceneShortsResponse {
    data: ShortItem[];
    total: number;
    page: number;
    limit: number;
    pending: ShortTask[];
}

export interface ShortsSettings {
    max_duration: number;
    save_dir: string | null;
    default_save_dir: string;
    effective_save_dir: string;
    storage_path: { id: number; name: string } | null;
    writable: boolean;
    problem?: string;
    search_default: ShortsSearchFilter;
}

export interface ShortEventData {
    task_id: string;
    source_scene_id: number;
    percent?: number;
    start?: number;
    end?: number;
    title?: string;
    scene_id?: number;
    error?: string;
}
