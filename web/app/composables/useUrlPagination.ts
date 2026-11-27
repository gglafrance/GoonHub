/**
 * Composable for syncing pagination state with URL query parameters.
 * Persists the current page in the URL so users return to the same page after refresh.
 *
 * @param defaultPage - The default page number (defaults to 1)
 * @returns A ref that syncs with the URL's `page` query parameter
 */
export function useUrlPagination(defaultPage = 1) {
    return useUrlQueryRef('page', defaultPage, (raw) => {
        const p = Number(raw);
        return Number.isInteger(p) && p > 0 ? p : undefined;
    });
}
