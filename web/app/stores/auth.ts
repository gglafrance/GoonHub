import { defineStore } from 'pinia';
import type { User } from '~/types/auth';

export const useAuthStore = defineStore(
    'auth',
    () => {
        const user = ref<User | null>(null);
        const token = ref<string | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);

        const setUser = (userData: User | null) => {
            user.value = userData;
        };

        const setToken = (authToken: string) => {
            token.value = authToken;
        };

        const clearUser = () => {
            user.value = null;
            token.value = null;
            error.value = null;
        };

        const clearToken = () => {
            token.value = null;
        };

        const setLoading = (loading: boolean) => {
            isLoading.value = loading;
        };

        const setError = (errorMessage: string | null) => {
            error.value = errorMessage;
        };

        const clearError = () => {
            error.value = null;
        };

        return {
            user,
            token,
            isLoading,
            error,
            setUser,
            setToken,
            clearUser,
            clearToken,
            setLoading,
            setError,
            clearError,
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

export interface AuthStoreState {
    user: User | null;
    token: string | null;
    isLoading: boolean;
    error: string | null;
}
