/**
 * Scroll helpers of an infinite scroll list.
 *
 * Restoring the position on Back/Forward or a reload is owned by the router scrollBehavior
 * (router.options.ts) and plugins/scroll-restore.client.ts, in both list modes: they wait until
 * the rebuilt list (up to INFINITE_REBUILD_PAGES pages, or the pages kept by the store) is tall
 * enough, so there is a single owner and no two restorations fighting each other.
 */
export function useInfiniteScrollPosition() {
    // Keeps the visible scenes in place while earlier ones are inserted above
    const keepPositionWhile = async (prepend: () => Promise<unknown>) => {
        if (!import.meta.client) return prepend();
        const before = document.documentElement.scrollHeight;
        await prepend();
        await nextTick();
        window.scrollBy({ top: document.documentElement.scrollHeight - before });
    };

    return { keepPositionWhile };
}
