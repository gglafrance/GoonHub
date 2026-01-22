export interface User {
    id: number;
    username: string;
    role: 'admin' | 'user';
}

export interface AuthResponse {
    token: string;
    user: User;
}

export interface LoginRequest {
    username: string;
    password: string;
}

export interface ErrorResponse {
    error: string;
}
