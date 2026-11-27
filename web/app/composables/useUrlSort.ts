/**
 * Composable for syncing sort state with URL query parameters.
 * Persists the selected sort in the URL so users return to the same sort after refresh.
 *
 * @param defaultSort - The default sort value (removed from URL when active)
 * @returns A ref that syncs with the URL's `sort` query parameter
 */
export function useUrlSort(defaultSort: string) {
    return useUrlString('sort', defaultSort);
}
