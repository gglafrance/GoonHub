import { defineStore } from 'pinia';
import type { User, AuthResponse, ErrorResponse } from '~/types/auth';

// Configuration constants
const VALIDATION_CACHE_WINDOW_MS = 5 * 60 * 1000; // 5 minutes

export const useAuthStore = defineStore(
    'auth',
    () => {
        const user = ref<User | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);

        // Validation tracking
        const lastValidatedAt = ref<number>(0);
        const isValidating = ref(false);

        // Cross-tab logout sync
        let logoutChannel: BroadcastChannel | null = null;

        // SECURITY: Token is stored in HTTP-only cookie only, not accessible from JS
        const isAuthenticated = computed(() => !!user.value);

        // True if validated within the cache window
        const isValidationFresh = computed(() => {
            if (lastValidatedAt.value === 0) return false;
            return Date.now() - lastValidatedAt.value < VALIDATION_CACHE_WINDOW_MS;
        });

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
                lastValidatedAt.value = Date.now();
                return data;
            } finally {
                isLoading.value = false;
            }
        };

        const logout = async (broadcast = true) => {
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
            lastValidatedAt.value = 0;

            // Notify other tabs about logout
            if (broadcast) {
                broadcastLogout();
            }

            navigateTo('/login');
        };

        // Cross-tab logout sync
        const initLogoutChannel = () => {
            if (!import.meta.client || logoutChannel) return;

            try {
                logoutChannel = new BroadcastChannel('auth-logout');
                logoutChannel.onmessage = (event) => {
                    if (event.data === 'logout') {
                        // Logout triggered from another tab - don't broadcast again
                        user.value = null;
                        error.value = null;
                        lastValidatedAt.value = 0;
                        navigateTo('/login');
                    }
                };
            } catch {
                // BroadcastChannel not supported, fallback to no cross-tab sync
            }
        };

        const broadcastLogout = () => {
            if (logoutChannel) {
                try {
                    logoutChannel.postMessage('logout');
                } catch {
                    // Channel may be closed
                }
            }
        };

        const destroyLogoutChannel = () => {
            if (logoutChannel) {
                logoutChannel.close();
                logoutChannel = null;
            }
        };

        const fetchCurrentUser = async (): Promise<User | null> => {
            // Use cookie for auth
            const response = await fetch('/api/v1/auth/me', {
                credentials: 'include', // Send HTTP-only cookie
            });

            if (!response.ok) {
                if (response.status === 401) {
                    user.value = null;
                    lastValidatedAt.value = 0;
                }
                return null;
            }

            const userData: User = await response.json();
            user.value = userData;
            lastValidatedAt.value = Date.now();
            return userData;
        };

        // Smart validation with caching - only validates if stale or forced
        const validateSession = async (force = false): Promise<boolean> => {
            // Skip if already validating
            if (isValidating.value) return !!user.value;

            // Skip if validation is fresh and not forced
            if (!force && isValidationFresh.value) return !!user.value;

            isValidating.value = true;
            try {
                const currentUser = await fetchCurrentUser();
                return !!currentUser;
            } catch {
                return false;
            } finally {
                isValidating.value = false;
            }
        };

        // Reset validation timestamp (called on 401 to force revalidation)
        const invalidateValidation = () => {
            lastValidatedAt.value = 0;
        };

        // Check if session is still valid on app startup
        const checkSession = async (): Promise<boolean> => {
            return validateSession(true);
        };

        return {
            user,
            isLoading,
            error,
            isAuthenticated,
            isValidationFresh,
            isValidating,
            lastValidatedAt,
            login,
            logout,
            fetchCurrentUser,
            validateSession,
            invalidateValidation,
            checkSession,
            initLogoutChannel,
            destroyLogoutChannel,
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
            // Persist user info and validation timestamp (token is in HTTP-only cookie)
            pick: ['user', 'lastValidatedAt'],
        },
    },
);
