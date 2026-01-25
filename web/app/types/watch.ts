import type { Video } from './video';

export interface UserVideoWatch {
    id: number;
    user_id: number;
    video_id: number;
    watched_at: string;
    watch_duration: number;
    last_position: number;
    completed: boolean;
    created_at: string;
    updated_at: string;
}

export interface WatchHistoryEntry {
    watch: UserVideoWatch;
    video?: Video;
}

export interface WatchHistoryResponse {
    entries: WatchHistoryEntry[];
    total: number;
    page: number;
    limit: number;
}

export interface VideoWatchesResponse {
    watches: UserVideoWatch[];
}

export interface ResumePositionResponse {
    position: number;
}
