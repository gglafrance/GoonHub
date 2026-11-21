/**
 * Scroll position of an infinite scroll list: saved when leaving the page (or its query/params
 * change through a navigation), restored once the list is rebuilt after a back navigation or a
 * reload. Only active while the infinite scroll setting is on.
 */
export function useInfiniteScrollPosition() {
    const settingsStore = useSettingsStore();
    const route = useRoute();

    const keyOf = (fullPath: string) => `infinite-scroll:${fullPath}`;

    const save = (fullPath: string) => {
        if (!import.meta.client || !settingsStore.infiniteScroll) return;
        sessionStorage.setItem(keyOf(fullPath), String(Math.round(window.scrollY)));
    };

    onBeforeRouteLeave((_to, from) => save(from.fullPath));
    onBeforeRouteUpdate((to, from) => {
        // The URL page changes while scrolling (replace): only save on a real navigation
        if (to.path !== from.path) save(from.fullPath);
    });

    // After a full rebuild of the list: back to where the user was on this URL
    const restore = () => {
        if (!import.meta.client || !settingsStore.infiniteScroll) return;
        const key = keyOf(route.fullPath);
        const saved = sessionStorage.getItem(key);
        if (saved === null) return;
        sessionStorage.removeItem(key);
        nextTick(() => requestAnimationFrame(() => window.scrollTo({ top: Number(saved) })));
    };

    // Keeps the visible scenes in place while earlier ones are inserted above
    const keepPositionWhile = async (prepend: () => Promise<unknown>) => {
        if (!import.meta.client) return prepend();
        const before = document.documentElement.scrollHeight;
        await prepend();
        await nextTick();
        window.scrollBy({ top: document.documentElement.scrollHeight - before });
    };

    return { restore, keepPositionWhile };
}
