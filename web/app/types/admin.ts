export interface AdminUser {
    id: number;
    username: string;
    role: string;
    created_at: string;
    last_login_at: string | null;
}

export interface AdminUsersResponse {
    users: AdminUser[];
    total: number;
    page: number;
    limit: number;
}

export interface RoleResponse {
    id: number;
    name: string;
    description: string;
    permissions: PermissionResponse[];
}

export interface PermissionResponse {
    id: number;
    name: string;
    description: string;
}
