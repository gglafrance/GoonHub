import { useAuthStore } from '~/stores/auth';
import type { AuthResponse, ErrorResponse } from '~/types/auth';

export const useAuth = () => {
    const authStore = useAuthStore();
    const router = useRouter();

    const login = async (username: string, password: string): Promise<AuthResponse> => {
        const response = await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ username, password }),
        });

        if (!response.ok) {
            const error: ErrorResponse = await response.json();
            throw new Error(error.error || 'Login failed');
        }

        const data: AuthResponse = await response.json();

        // Store token and user
        sessionStorage.setItem('auth_token', data.token);
        sessionStorage.setItem('auth_timestamp', Date.now().toString());
        authStore.setUser(data.user);
        authStore.setToken(data.token);

        return data;
    };

    const logout = () => {
        sessionStorage.removeItem('auth_token');
        sessionStorage.removeItem('auth_timestamp');
        authStore.clearUser();
        authStore.clearToken();
        router.push('/login');
    };

    const isAuthenticated = (): boolean => {
        return !!sessionStorage.getItem('auth_token') && !!authStore.user;
    };

    const getToken = (): string | null => {
        // Check both sessionStorage and store
        return sessionStorage.getItem('auth_token') || authStore.token;
    };

    const fetchCurrentUser = async () => {
        const token = getToken();
        if (!token) return null;

        const response = await fetch('/api/v1/auth/me', {
            headers: {
                Authorization: `Bearer ${token}`,
            },
        });

        if (!response.ok) {
            if (response.status === 401) {
                logout();
            }
            throw new Error('Failed to fetch current user');
        }

        const user = await response.json();
        authStore.setUser(user);

        // Ensure token is in store (for persistence and component checks)
        if (!authStore.token && token) {
            authStore.setToken(token);
        }

        return user;
    };

    return {
        login,
        logout,
        isAuthenticated,
        getToken,
        fetchCurrentUser,
    };
};
