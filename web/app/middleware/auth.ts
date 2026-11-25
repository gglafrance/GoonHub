import type { PermissionName } from '~/types/permissions';

// Page that needs a permission the user lacks: back to the homepage
const deniedRedirect = (
    required: PermissionName[] | undefined,
    permissions: ReturnType<typeof useMyPermissions>,
) => {
    if (!required || required.length === 0 || permissions.hasAny(...required)) return undefined;
    return navigateTo('/');
};

export default defineNuxtRouteMiddleware(async (to, from) => {
    const authStore = useAuthStore();
    // Permissions gate the page blocks synchronously: loaded before the page renders (once per
    // user, a no-op afterwards), refreshed after login and in the background (they are live)
    const permissions = useMyPermissions();

    const isPublicRoute = ['/login'].includes(to.path);

    if (!isPublicRoute) {
        // Determine if this is initial load (from server or direct URL access)
        // vs in-app navigation (clicking links within the app)
        // Note: !from.name means no previous route (initial page load)
        // Query-only changes (same path) are NOT initial loads
        const isInitialLoad = !from.name;

        // Case 1: No user in store at all - redirect to login
        if (!authStore.user) {
            return navigateTo({
                path: '/login',
                query: { redirect: to.fullPath },
            });
        }

        // Case 2: Initial load (direct URL or refresh) - always validate (blocking)
        if (isInitialLoad) {
            try {
                const user = await authStore.validateSession(true);
                if (!user) {
                    authStore.$patch({ user: null });
                    return navigateTo({
                        path: '/login',
                        query: { redirect: to.fullPath },
                    });
                }
                // Always fetch fresh settings on initial load (sessionStorage may be stale from another tab)
                const settingsStore = useSettingsStore();
                await Promise.all([settingsStore.loadSettings(), permissions.load(true)]);
            } catch {
                authStore.$patch({ user: null });
                return navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
            return deniedRedirect(to.meta.permissions, permissions);
        }

        // Coming from the login page: another user may have signed in
        const permissionsLoad = permissions.load(from.path === '/login');

        // Case 3: In-app navigation with fresh validation - navigate immediately
        if (authStore.isValidationFresh) {
            // Trust cached user, ensure settings loaded
            const settingsStore = useSettingsStore();
            if (!settingsStore.settings) {
                await settingsStore.loadSettings();
            }
            await permissionsLoad;
            return deniedRedirect(to.meta.permissions, permissions);
        }

        // Case 4: In-app navigation with stale validation - navigate immediately, validate in background
        // Trust cached user for instant navigation
        const settingsStore = useSettingsStore();
        if (!settingsStore.settings) {
            await settingsStore.loadSettings();
        }
        await permissionsLoad;

        // Validate in background (non-blocking), then refresh the permissions (they are live)
        authStore.validateSession().then((isValid) => {
            if (isValid) {
                void permissions.load(true);
            } else {
                // Session invalid, redirect to login
                navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
        });
        return deniedRedirect(to.meta.permissions, permissions);
    }

    // If already logged in and trying to access login page, redirect to home
    if (authStore.user && to.path === '/login') {
        const redirect = to.query.redirect as string;
        return navigateTo(redirect || '/');
    }
});
