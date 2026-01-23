import { defineStore } from 'pinia';
import type { User, AuthResponse, ErrorResponse } from '~/types/auth';

export const useAuthStore = defineStore(
    'auth',
    () => {
        const user = ref<User | null>(null);
        const token = ref<string | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);

        const isAuthenticated = computed(() => !!token.value && !!user.value);

        const login = async (username: string, password: string): Promise<AuthResponse> => {
            isLoading.value = true;
            error.value = null;

            try {
                const response = await fetch('/api/v1/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password }),
                });

                if (!response.ok) {
                    const err: ErrorResponse = await response.json();
                    throw new Error(err.error || 'Login failed');
                }

                const data: AuthResponse = await response.json();
                token.value = data.token;
                user.value = data.user;
                return data;
            } finally {
                isLoading.value = false;
            }
        };

        const logout = async () => {
            try {
                if (token.value) {
                    await fetch('/api/v1/auth/logout', {
                        method: 'POST',
                        headers: { Authorization: `Bearer ${token.value}` },
                    });
                }
            } catch (e: unknown) {
                console.error('Logout API call failed:', e);
            }

            token.value = null;
            user.value = null;
            error.value = null;
            navigateTo('/login');
        };

        const fetchCurrentUser = async (): Promise<User | null> => {
            if (!token.value) return null;

            const response = await fetch('/api/v1/auth/me', {
                headers: { Authorization: `Bearer ${token.value}` },
            });

            if (!response.ok) {
                if (response.status === 401) {
                    token.value = null;
                    user.value = null;
                }
                throw new Error('Failed to fetch current user');
            }

            const userData: User = await response.json();
            user.value = userData;
            return userData;
        };

        return {
            user,
            token,
            isLoading,
            error,
            isAuthenticated,
            login,
            logout,
            fetchCurrentUser,
        };
    },
    {
        persist: {
            key: 'auth-store',
            storage: {
                getItem: (key) => {
                    if (import.meta.client) {
                        return sessionStorage.getItem(key);
                    }
                    return null;
                },
                setItem: (key, value) => {
                    if (import.meta.client) {
                        sessionStorage.setItem(key, value);
                    }
                },
            },
            pick: ['user', 'token'],
        },
    },
);
