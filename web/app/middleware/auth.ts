export default defineNuxtRouteMiddleware(async (to, from) => {
    const authStore = useAuthStore();

    const isPublicRoute = ['/login'].includes(to.path);

    if (!isPublicRoute) {
        // Determine if this is initial load (from server or direct URL access)
        // vs in-app navigation (clicking links within the app)
        const isInitialLoad = !from.name || from.path === to.path;

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
                // Load settings if not already loaded
                const settingsStore = useSettingsStore();
                if (!settingsStore.settings) {
                    await settingsStore.loadSettings();
                }
            } catch {
                authStore.$patch({ user: null });
                return navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
            return;
        }

        // Case 3: In-app navigation with fresh validation - navigate immediately
        if (authStore.isValidationFresh) {
            // Trust cached user, ensure settings loaded
            const settingsStore = useSettingsStore();
            if (!settingsStore.settings) {
                await settingsStore.loadSettings();
            }
            return;
        }

        // Case 4: In-app navigation with stale validation - navigate immediately, validate in background
        // Trust cached user for instant navigation
        const settingsStore = useSettingsStore();
        if (!settingsStore.settings) {
            await settingsStore.loadSettings();
        }

        // Validate in background (non-blocking)
        authStore.validateSession().then((isValid) => {
            if (!isValid) {
                // Session invalid, redirect to login
                navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
        });
    }

    // If already logged in and trying to access login page, redirect to home
    if (authStore.user && to.path === '/login') {
        const redirect = to.query.redirect as string;
        return navigateTo(redirect || '/');
    }
});
