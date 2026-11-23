// Trash page (Settings > Trash): GET /api/v1/admin/trash and the bulk endpoints.

export type TrashSource = 'duplicates' | 'other';

export type TrashSort = 'trashed_at' | 'size' | 'duration' | 'title' | 'purge_at';

export interface TrashedScene {
    id: number;
    title: string;
    thumbnail_path: string;
    stored_path: string;
    storage_path_id: number | null;
    storage_path_name: string;
    relative_path: string;
    size: number;
    duration: number;
    width: number;
    height: number;
    video_codec: string;
    trashed_at: string;
    /** When the scene will be purged: the retention, or a sooner scheduled duplicate deletion */
    expires_at: string;
    scheduled_deletion_at?: string;
    source: TrashSource;
    duplicate_group_id?: number;
    /** Under a read-only storage path: a permanent deletion keeps the file */
    kept_on_disk: boolean;
    /** Found missing by the last completed missing-file check */
    file_missing: boolean;
}

export interface TrashTotals {
    count: number;
    size: number;
}

export interface TrashListResponse {
    data: TrashedScene[];
    total: number;
    page: number;
    limit: number;
    retention_days: number;
    pagination: { page: number; limit: number; total_items: number; total_pages: number };
    totals: { filtered: TrashTotals; all: TrashTotals };
}

export interface TrashIdsResponse {
    ids: number[];
    items: { id: number; size: number }[];
    total: number;
    limit: number;
}

export interface TrashFacets {
    codecs: { codec: string; count: number }[];
    storage_paths: {
        storage_path_id: number | null;
        name: string | null;
        count: number;
        size: number;
    }[];
    duplicates: number;
}

export interface TrashBulkResult {
    succeeded: number[];
    failed: { scene_id: number; error: string }[];
    size: number;
}
