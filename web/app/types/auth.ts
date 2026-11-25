export interface User {
    id: number;
    username: string;
    // Role name from the RBAC roles table. Access checks use the permissions
    // (GET /auth/permissions, useMyPermissions), never the role name
    role: string;
}

// SECURITY: Token is transmitted only via HTTP-only cookie, never in response body
export interface AuthResponse {
    user: User;
}

export interface LoginRequest {
    username: string;
    password: string;
}

export interface ErrorResponse {
    error: string;
}
