import { defineStore } from 'pinia';
import type { User, AuthResponse, ErrorResponse } from '~/types/auth';

export const useAuthStore = defineStore(
    'auth',
    () => {
        const user = ref<User | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);

        // SECURITY: Token is stored in HTTP-only cookie only, not accessible from JS
        const isAuthenticated = computed(() => !!user.value);

        const login = async (username: string, password: string): Promise<AuthResponse> => {
            isLoading.value = true;
            error.value = null;

            try {
                const response = await fetch('/api/v1/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'include', // Include cookies in request
                    body: JSON.stringify({ username, password }),
                });

                if (!response.ok) {
                    const err: ErrorResponse = await response.json();
                    throw new Error(err.error || 'Login failed');
                }

                const data: AuthResponse = await response.json();
                // Token is set in HTTP-only cookie by server (not in response body)
                user.value = data.user;
                return data;
            } finally {
                isLoading.value = false;
            }
        };

        const logout = async () => {
            try {
                // Server will clear the HTTP-only cookie
                await fetch('/api/v1/auth/logout', {
                    method: 'POST',
                    credentials: 'include', // Send cookie for auth
                });
            } catch (e: unknown) {
                console.error('Logout API call failed:', e);
            }

            user.value = null;
            error.value = null;
            navigateTo('/login');
        };

        const fetchCurrentUser = async (): Promise<User | null> => {
            // Use cookie for auth
            const response = await fetch('/api/v1/auth/me', {
                credentials: 'include', // Send HTTP-only cookie
            });

            if (!response.ok) {
                if (response.status === 401) {
                    user.value = null;
                }
                return null;
            }

            const userData: User = await response.json();
            user.value = userData;
            return userData;
        };

        // Check if session is still valid on app startup
        const checkSession = async (): Promise<boolean> => {
            try {
                const currentUser = await fetchCurrentUser();
                return !!currentUser;
            } catch {
                return false;
            }
        };

        return {
            user,
            isLoading,
            error,
            isAuthenticated,
            login,
            logout,
            fetchCurrentUser,
            checkSession,
        };
    },
    {
        persist: {
            key: 'auth-store',
            storage: {
                getItem: (key: string) => {
                    if (import.meta.client) {
                        return localStorage.getItem(key);
                    }
                    return null;
                },
                setItem: (key: string, value: string) => {
                    if (import.meta.client) {
                        localStorage.setItem(key, value);
                    }
                },
            },
            // Only persist user info (token is in HTTP-only cookie)
            pick: ['user'],
        },
    },
);
