/**
 * Composable for syncing sort state with URL query parameters.
 * Persists the selected sort in the URL so users return to the same sort after refresh.
 *
 * @param defaultSort - The default sort value (removed from URL when active)
 * @returns A ref that syncs with the URL's `sort` query parameter
 */
export function useUrlSort(defaultSort: string) {
    const route = useRoute();
    const router = useRouter();

    function getSortFromQuery(): string {
        const s = route.query.sort;
        return typeof s === 'string' && s ? s : defaultSort;
    }

    // Initialize from URL or default
    const sort = ref(getSortFromQuery());

    // Sync URL when sort changes
    watch(sort, (newSort) => {
        const query = { ...route.query };
        if (newSort === defaultSort) {
            delete query.sort;
        } else {
            query.sort = newSort;
        }
        router.replace({ query });
    });

    // Handle browser back/forward navigation
    watch(
        () => route.query.sort,
        () => {
            const urlSort = getSortFromQuery();
            if (sort.value !== urlSort) {
                sort.value = urlSort;
            }
        },
    );

    return sort;
}
