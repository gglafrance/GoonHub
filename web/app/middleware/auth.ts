export default defineNuxtRouteMiddleware(async (to) => {
    const authStore = useAuthStore();

    const isPublicRoute = ['/login'].includes(to.path);

    if (!isPublicRoute) {
        if (!authStore.token) {
            return navigateTo({
                path: '/login',
                query: { redirect: to.fullPath },
            });
        }

        if (!authStore.user) {
            try {
                await authStore.fetchCurrentUser();
                const settingsStore = useSettingsStore();
                if (!settingsStore.settings) {
                    await settingsStore.loadSettings();
                }
            } catch (e: unknown) {
                authStore.$patch({ token: null, user: null });
                return navigateTo({
                    path: '/login',
                    query: { redirect: to.fullPath },
                });
            }
        }

        if (!authStore.user) {
            return navigateTo({
                path: '/login',
                query: { redirect: to.fullPath },
            });
        }
    }

    if (authStore.user && to.path === '/login') {
        const redirect = to.query.redirect as string;
        return navigateTo(redirect || '/');
    }
});
