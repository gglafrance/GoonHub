/**
 * Composable for visibility-based auth validation.
 * Validates session in background when tab regains focus after being hidden.
 */

// Minimum time tab must be hidden before triggering revalidation on focus
const MIN_HIDDEN_TIME_MS = 30 * 1000; // 30 seconds

export const useAuthValidation = () => {
    const authStore = useAuthStore();
    const router = useRouter();

    let lastHiddenAt: number | null = null;
    let isInitialized = false;

    const handleVisibilityChange = async () => {
        if (document.hidden) {
            // Tab became hidden - record timestamp
            lastHiddenAt = Date.now();
        } else {
            // Tab became visible - check if we should revalidate
            if (lastHiddenAt && Date.now() - lastHiddenAt > MIN_HIDDEN_TIME_MS) {
                // Tab was hidden for >30s, validate in background
                if (authStore.user) {
                    const isValid = await authStore.validateSession();
                    if (!isValid) {
                        // Session invalid, redirect to login
                        router.push({
                            path: '/login',
                            query: { redirect: router.currentRoute.value.fullPath },
                        });
                    }
                }
            }
            lastHiddenAt = null;
        }
    };

    const startAuthValidation = () => {
        if (!import.meta.client || isInitialized) return;

        isInitialized = true;

        // Initialize cross-tab logout sync
        authStore.initLogoutChannel();

        // Listen for visibility changes
        document.addEventListener('visibilitychange', handleVisibilityChange);
    };

    const stopAuthValidation = () => {
        if (!import.meta.client || !isInitialized) return;

        isInitialized = false;

        // Cleanup cross-tab logout channel
        authStore.destroyLogoutChannel();

        // Remove visibility listener
        document.removeEventListener('visibilitychange', handleVisibilityChange);
    };

    return {
        startAuthValidation,
        stopAuthValidation,
    };
};
