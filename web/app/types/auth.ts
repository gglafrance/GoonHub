export interface User {
    id: number;
    username: string;
    role: 'admin' | 'user';
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
