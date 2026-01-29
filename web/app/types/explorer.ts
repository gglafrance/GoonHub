import type { Video } from './video';
import type { StoragePath } from './storage';

export interface FolderInfo {
    name: string;
    path: string;
    video_count: number;
    total_duration: number;
    total_size: number;
}

export interface StoragePathWithCount extends StoragePath {
    video_count: number;
}

export interface StoragePathsResponse {
    storage_paths: StoragePathWithCount[];
}

export interface FolderContentsResponse {
    storage_path: StoragePath;
    current_path: string;
    subfolders: FolderInfo[];
    videos: Video[];
    total_videos: number;
    page: number;
    limit: number;
}

export interface BulkUpdateTagsRequest {
    video_ids: number[];
    tag_ids: number[];
    mode: 'add' | 'remove' | 'replace';
}

export interface BulkUpdateActorsRequest {
    video_ids: number[];
    actor_ids: number[];
    mode: 'add' | 'remove' | 'replace';
}

export interface BulkUpdateStudioRequest {
    video_ids: number[];
    studio: string;
}

export interface FolderVideoIDsRequest {
    storage_path_id: number;
    folder_path: string;
    recursive: boolean;
}

export interface BulkUpdateResponse {
    updated: number;
    requested: number;
}

export interface FolderVideoIDsResponse {
    video_ids: number[];
    count: number;
}

export interface BulkDeleteRequest {
    video_ids: number[];
}

export interface BulkDeleteResponse {
    deleted: number;
    requested: number;
}

export interface FolderSearchRequest {
    storage_path_id: number;
    folder_path: string;
    recursive: boolean;
    query: string;
    tag_ids?: number[];
    actors?: string[];
    studio?: string;
    min_duration?: number;
    max_duration?: number;
    sort?: string;
    page: number;
    limit: number;
}

export interface FolderSearchResponse {
    videos: import('./video').Video[];
    total: number;
    page: number;
    limit: number;
}
