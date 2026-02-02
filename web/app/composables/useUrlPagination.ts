/**
 * Composable for syncing pagination state with URL query parameters.
 * Persists the current page in the URL so users return to the same page after refresh.
 *
 * @param defaultPage - The default page number (defaults to 1)
 * @returns A ref that syncs with the URL's `page` query parameter
 */
export function useUrlPagination(defaultPage = 1) {
    const route = useRoute();
    const router = useRouter();

    function getPageFromQuery(): number {
        const p = Number(route.query.page);
        return p > 0 ? p : defaultPage;
    }

    // Initialize from URL or default
    const page = ref(getPageFromQuery());

    // Sync URL when page changes
    watch(page, (newPage) => {
        const query = { ...route.query };
        if (newPage === defaultPage) {
            delete query.page;
        } else {
            query.page = String(newPage);
        }
        router.replace({ query });
    });

    // Handle browser back/forward navigation
    watch(
        () => route.query.page,
        () => {
            const urlPage = getPageFromQuery();
            if (page.value !== urlPage) {
                page.value = urlPage;
            }
        },
    );

    return page;
}
