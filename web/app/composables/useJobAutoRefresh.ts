/**
 * Composable for auto-refresh toggle with localStorage persistence.
 */
export const useJobAutoRefresh = (options: { onRefresh: () => void; intervalMs?: number }) => {
    const autoRefresh = ref(localStorage.getItem('jobs-auto-refresh') === 'true');
    const autoRefreshInterval = ref<ReturnType<typeof setInterval> | null>(null);

    const intervalMs = options.intervalMs ?? 5000;

    const cleanup = () => {
        if (autoRefreshInterval.value) {
            clearInterval(autoRefreshInterval.value);
            autoRefreshInterval.value = null;
        }
    };

    watch(
        autoRefresh,
        (enabled) => {
            localStorage.setItem('jobs-auto-refresh', String(enabled));
            cleanup();
            if (enabled) {
                autoRefreshInterval.value = setInterval(options.onRefresh, intervalMs);
            }
        },
        { immediate: true },
    );

    onUnmounted(cleanup);

    const toggle = () => {
        autoRefresh.value = !autoRefresh.value;
    };

    return {
        autoRefresh,
        toggle,
    };
};
