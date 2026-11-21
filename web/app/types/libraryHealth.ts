import type { DiskUsage } from '~/types/storage';

export type HealthCategory =
    | 'missing_file'
    | 'unreadable_file'
    | 'corrupted'
    | 'size_zero'
    | 'duration_zero'
    | 'unplayable'
    | 'no_thumbnail'
    | 'no_sprites'
    | 'no_preview'
    | 'no_fingerprint'
    | 'no_storage_path'
    | 'outside_storage_path';

export type HealthPhase =
    | 'metadata'
    | 'thumbnail'
    | 'sprites'
    | 'animated_thumbnails'
    | 'fingerprint';

export type HealthTaskKind = 'missing_files' | 'fix_sizes';

/** What an action button of the health page does: queue the missing jobs of a phase, or run a task */
export type HealthAction = HealthPhase | HealthTaskKind;

export interface HealthCheck {
    id: number;
    kind: HealthTaskKind;
    status: 'running' | 'completed' | 'cancelled' | 'failed';
    started_at: string;
    completed_at: string | null;
    total: number;
    checked: number;
    missing: number;
    unreadable: number;
    fixed: number;
    unchanged: number;
    error_message?: string;
    created_at: string;
}

export interface HealthTaskStatus {
    running: boolean;
    task?: HealthCheck;
}

export interface StoragePathHealth {
    id: number | null;
    name: string;
    path: string;
    scene_count: number;
    total_size: number;
    disk_usage: DiskUsage | null;
}

export interface LibraryHealthSummary {
    total_scenes: number;
    counts: Partial<Record<HealthCategory, number>>;
    storage_paths: StoragePathHealth[];
    last_missing_check: HealthCheck | null;
    last_fix_sizes: HealthCheck | null;
    task: HealthTaskStatus;
    fingerprint_enabled: boolean;
    scene_preview_enabled: boolean;
}

export interface HealthSceneRow {
    id: number;
    title: string;
    stored_path: string;
    size: number;
    duration: number;
    video_codec: string;
    audio_codec: string;
    storage_path_id: number | null;
    is_corrupted: boolean;
    created_at: string;
    detail?: string;
}

export interface HealthSceneList {
    data: HealthSceneRow[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}

export interface HealthActionPreview {
    action: HealthAction;
    count: number;
    enabled: boolean;
    reason?: string;
}
