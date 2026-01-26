export default defineNuxtRouteMiddleware(async (to) => {
    const authStore = useAuthStore();

    const isPublicRoute = ['/login'].includes(to.path);

    if (!isPublicRoute) {
        // With HTTP-only cookies, verify session by calling the API.
        // The cookie is sent automatically with credentials: 'include'.
        try {
            const user = await authStore.fetchCurrentUser();
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
    }

    // If already logged in and trying to access login page, redirect to home
    if (authStore.user && to.path === '/login') {
        const redirect = to.query.redirect as string;
        return navigateTo(redirect || '/');
    }
});
