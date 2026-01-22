import { useAuth } from '~/composables/useAuth';
import { useAuthStore } from '~/stores/auth';

export default defineNuxtRouteMiddleware(async (to) => {
    const authComposable = useAuth();
    const authStore = useAuthStore();

    const isPublicRoute = ['/login'].includes(to.path);

    if (!isPublicRoute) {
        // Check sessionStorage directly first (bypasses store hydration delay)
        const token = sessionStorage.getItem('auth_token');

        if (!token) {
            return navigateTo({
                path: '/login',
                query: { redirect: to.fullPath },
            });
        }

        // If we have a token but no user in store, try to fetch current user
        if (!authStore.user && token) {
            try {
                await authComposable.fetchCurrentUser();
            } catch (error) {
                sessionStorage.removeItem('auth_token');
                sessionStorage.removeItem('auth_timestamp');
                return navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
        }

        // Sync token to store if missing (handles hydration race/stale state)
        if (token && !authStore.token) {
            authStore.setToken(token);
        }

        // If still no user after fetch, redirect to login
        if (!authStore.user) {
            return navigateTo({
                path: '/login',
                query: { redirect: to.fullPath },
            });
        }
    }

    // Redirect authenticated users away from login page
    if (authStore.user && to.path === '/login') {
        const redirect = to.query.redirect as string;
        return navigateTo(redirect || '/');
    }
});
