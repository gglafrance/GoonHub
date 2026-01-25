export interface StoragePath {
    id: number;
    name: string;
    path: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
}

export interface StoragePathListResponse {
    storage_paths: StoragePath[];
}

export interface ValidatePathResponse {
    valid: boolean;
    message: string;
}
