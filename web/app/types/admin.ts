export interface AdminUser {
    id: number;
    username: string;
    role: string;
    // The user's markers are visible (read-only) to every user
    share_markers: boolean;
    created_at: string;
    last_login_at: string | null;
}

export interface AdminUsersResponse {
    users: AdminUser[];
    total: number;
    page: number;
    limit: number;
}

// Cache of the on-the-fly conversions (GET /admin/transcode-cache)
export interface TranscodeCacheStats {
    enabled: boolean;
    size_bytes: number;
    entries: number;
    in_progress: number;
    max_size_bytes: number;
    max_age_seconds: number;
}

export interface TranscodeCacheClearResponse extends TranscodeCacheStats {
    removed: number;
    removed_bytes: number;
}
