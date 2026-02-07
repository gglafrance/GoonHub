export interface DiskUsage {
    total_bytes: number;
    used_bytes: number;
    free_bytes: number;
    used_pct: number;
}

export interface StoragePath {
    id: number;
    name: string;
    path: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
    disk_usage: DiskUsage | null;
}

export interface StoragePathListResponse {
    storage_paths: StoragePath[];
}

export interface ValidatePathResponse {
    valid: boolean;
    message: string;
}
