import type { SceneListItem } from './scene';

export interface UserSceneWatch {
    id: number;
    user_id: number;
    scene_id: number;
    watched_at: string;
    watch_duration: number;
    last_position: number;
    completed: boolean;
    created_at: string;
    updated_at: string;
}

export interface WatchHistoryEntry {
    watch: UserSceneWatch;
    scene?: SceneListItem;
}

export interface WatchHistoryResponse {
    entries: WatchHistoryEntry[];
    total: number;
    page: number;
    limit: number;
}

export interface SceneWatchesResponse {
    watches: UserSceneWatch[];
}

export interface ResumePositionResponse {
    position: number;
}

export interface DailyActivityCount {
    date: string;
    count: number;
}

export interface DateGroup {
    dateKey: string;
    date: string;
    entries: WatchHistoryEntry[];
}

export interface ChartActivityCount {
    dateKey: string;
    count: number;
}
