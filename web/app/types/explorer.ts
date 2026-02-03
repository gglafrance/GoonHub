import type { SceneListItem } from './scene';
import type { StoragePath } from './storage';

export interface FolderInfo {
    name: string;
    path: string;
    scene_count: number;
    total_duration: number;
    total_size: number;
}

export interface StoragePathWithCount extends StoragePath {
    scene_count: number;
}

export interface StoragePathsResponse {
    storage_paths: StoragePathWithCount[];
}

export interface FolderContentsResponse {
    storage_path: StoragePath;
    current_path: string;
    subfolders: FolderInfo[];
    scenes: SceneListItem[];
    total_scenes: number;
    page: number;
    limit: number;
}

export interface BulkUpdateTagsRequest {
    scene_ids: number[];
    tag_ids: number[];
    mode: 'add' | 'remove' | 'replace';
}

export interface BulkUpdateActorsRequest {
    scene_ids: number[];
    actor_ids: number[];
    mode: 'add' | 'remove' | 'replace';
}

export interface BulkUpdateStudioRequest {
    scene_ids: number[];
    studio: string;
}

export interface FolderSceneIDsRequest {
    storage_path_id: number;
    folder_path: string;
    recursive: boolean;
}

export interface BulkUpdateResponse {
    updated: number;
    requested: number;
}

export interface FolderSceneIDsResponse {
    scene_ids: number[];
    count: number;
}

export interface BulkDeleteRequest {
    scene_ids: number[];
    permanent?: boolean;
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
    scenes: SceneListItem[];
    total: number;
    page: number;
    limit: number;
}

export interface ScenesMatchInfoRequest {
    scene_ids: number[];
}

export interface SceneMatchInfo {
    id: number;
    title: string;
    original_filename: string;
    porndb_scene_id: string | null;
    actors: string[];
    studio: string | null;
    thumbnail_path: string;
    duration: number;
}

export interface ScenesMatchInfoResponse {
    scenes: SceneMatchInfo[];
}
