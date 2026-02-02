/**
 * Composable for fetching metadata from external sources with race condition handling.
 * Provides request cancellation, stale response detection, caching, and hover pre-fetching.
 */
export interface UseFetchMetadataOptions<TSearchResult, TDetails> {
    searchFn: (query: string, signal: AbortSignal) => Promise<TSearchResult[]>;
    fetchDetailsFn: (id: string, signal: AbortSignal) => Promise<TDetails>;
    getItemId: (item: TSearchResult) => string;
}

interface CacheEntry<T> {
    data: T;
    timestamp: number;
}

const CACHE_TTL = 5 * 60 * 1000; // 5 minutes
const MAX_CACHE_SIZE = 50;
const PREFETCH_DEBOUNCE = 200; // ms

export const useFetchMetadata = <TSearchResult, TDetails>(
    options: UseFetchMetadataOptions<TSearchResult, TDetails>,
) => {
    const { searchFn, fetchDetailsFn, getItemId } = options;

    // Search state
    const searchQuery = ref('');
    const isSearching = ref(false);
    const searchResults = ref<TSearchResult[]>([]) as Ref<TSearchResult[]>;
    const searchError = ref<string | null>(null);

    // Details state
    const selectedItem = ref<TSearchResult | null>(null) as Ref<TSearchResult | null>;
    const isFetchingDetails = ref(false);
    const itemDetails = ref<TDetails | null>(null) as Ref<TDetails | null>;
    const detailsError = ref<string | null>(null);

    // Prefetch state
    const prefetchingId = ref<string | null>(null);

    // Request tracking for race condition handling
    let searchAbortController: AbortController | null = null;
    let detailsAbortController: AbortController | null = null;
    let prefetchAbortController: AbortController | null = null;
    let searchRequestId = 0;
    let detailsRequestId = 0;
    let prefetchTimeout: ReturnType<typeof setTimeout> | null = null;

    // LRU cache for details
    const detailsCache = new Map<string, CacheEntry<TDetails>>();

    /**
     * Evict oldest entries if cache exceeds max size
     */
    const evictOldestCacheEntries = () => {
        if (detailsCache.size <= MAX_CACHE_SIZE) return;

        const entries = Array.from(detailsCache.entries());
        entries.sort((a, b) => a[1].timestamp - b[1].timestamp);

        const toRemove = entries.slice(0, entries.length - MAX_CACHE_SIZE);
        for (const [key] of toRemove) {
            detailsCache.delete(key);
        }
    };

    /**
     * Get cached details if fresh, otherwise return null
     */
    const getCachedDetails = (id: string): TDetails | null => {
        const entry = detailsCache.get(id);
        if (!entry) return null;

        const isExpired = Date.now() - entry.timestamp > CACHE_TTL;
        if (isExpired) {
            detailsCache.delete(id);
            return null;
        }

        // Update timestamp for LRU behavior
        entry.timestamp = Date.now();
        return entry.data;
    };

    /**
     * Store details in cache
     */
    const setCachedDetails = (id: string, data: TDetails) => {
        detailsCache.set(id, {
            data,
            timestamp: Date.now(),
        });
        evictOldestCacheEntries();
    };

    /**
     * Search for items. Cancels any pending search request.
     */
    const search = async (query: string) => {
        const trimmedQuery = query.trim();
        if (!trimmedQuery) return;

        // Cancel any pending search
        searchAbortController?.abort();
        searchAbortController = new AbortController();

        // Track request ID to ignore stale responses
        const currentRequestId = ++searchRequestId;

        isSearching.value = true;
        searchError.value = null;
        searchResults.value = [];
        selectedItem.value = null;
        itemDetails.value = null;

        try {
            const results = await searchFn(trimmedQuery, searchAbortController.signal);

            // Ignore stale response
            if (currentRequestId !== searchRequestId) return;

            searchResults.value = results;

            if (results.length === 0) {
                searchError.value = 'No results found';
            }
        } catch (err) {
            // Ignore aborted requests
            if (err instanceof Error && err.name === 'AbortError') return;

            // Ignore stale response errors
            if (currentRequestId !== searchRequestId) return;

            searchError.value = err instanceof Error ? err.message : 'Search failed';
        } finally {
            // Only update loading state if this is still the current request
            if (currentRequestId === searchRequestId) {
                isSearching.value = false;
            }
        }
    };

    /**
     * Fetch details for an item. Uses cache if available.
     * Includes click guard to prevent duplicate requests.
     */
    const fetchDetails = async (item: TSearchResult) => {
        // Click guard - prevent duplicate requests
        if (isFetchingDetails.value) return;

        const itemId = getItemId(item);

        // Cancel any pending details fetch
        detailsAbortController?.abort();
        detailsAbortController = new AbortController();

        // Cancel any pending prefetch
        if (prefetchTimeout) {
            clearTimeout(prefetchTimeout);
            prefetchTimeout = null;
        }
        prefetchAbortController?.abort();
        prefetchingId.value = null;

        // Track request ID to ignore stale responses
        const currentRequestId = ++detailsRequestId;

        selectedItem.value = item;
        isFetchingDetails.value = true;
        detailsError.value = null;
        itemDetails.value = null;

        // Check cache first
        const cached = getCachedDetails(itemId);
        if (cached) {
            itemDetails.value = cached;
            isFetchingDetails.value = false;
            return;
        }

        try {
            const details = await fetchDetailsFn(itemId, detailsAbortController.signal);

            // Ignore stale response
            if (currentRequestId !== detailsRequestId) return;

            itemDetails.value = details;
            setCachedDetails(itemId, details);
        } catch (err) {
            // Ignore aborted requests
            if (err instanceof Error && err.name === 'AbortError') return;

            // Ignore stale response errors
            if (currentRequestId !== detailsRequestId) return;

            detailsError.value = err instanceof Error ? err.message : 'Failed to fetch details';
        } finally {
            // Only update loading state if this is still the current request
            if (currentRequestId === detailsRequestId) {
                isFetchingDetails.value = false;
            }
        }
    };

    /**
     * Handle hover on an item - debounced prefetch
     */
    const handleHover = (item: TSearchResult) => {
        const itemId = getItemId(item);

        // Already fetching details for this item, skip prefetch
        if (selectedItem.value && getItemId(selectedItem.value) === itemId) return;

        // Already cached, no need to prefetch
        if (getCachedDetails(itemId)) return;

        // Already prefetching this item
        if (prefetchingId.value === itemId) return;

        // Clear any pending prefetch
        if (prefetchTimeout) {
            clearTimeout(prefetchTimeout);
        }
        prefetchAbortController?.abort();

        // Debounce the prefetch
        prefetchTimeout = setTimeout(async () => {
            prefetchAbortController = new AbortController();
            prefetchingId.value = itemId;

            try {
                const details = await fetchDetailsFn(itemId, prefetchAbortController.signal);
                setCachedDetails(itemId, details);
            } catch {
                // Silently fail prefetch - it's just an optimization
            } finally {
                if (prefetchingId.value === itemId) {
                    prefetchingId.value = null;
                }
            }
        }, PREFETCH_DEBOUNCE);
    };

    /**
     * Handle hover leave - cancel pending prefetch
     */
    const handleHoverLeave = () => {
        if (prefetchTimeout) {
            clearTimeout(prefetchTimeout);
            prefetchTimeout = null;
        }
        prefetchAbortController?.abort();
        prefetchingId.value = null;
    };

    /**
     * Reset state for a new search session
     */
    const reset = () => {
        searchQuery.value = '';
        isSearching.value = false;
        searchResults.value = [];
        searchError.value = null;
        selectedItem.value = null;
        isFetchingDetails.value = false;
        itemDetails.value = null;
        detailsError.value = null;
        prefetchingId.value = null;
    };

    /**
     * Go back to search results
     */
    const goBack = () => {
        selectedItem.value = null;
        itemDetails.value = null;
        detailsError.value = null;
    };

    /**
     * Cleanup - abort all pending requests
     */
    const cleanup = () => {
        searchAbortController?.abort();
        detailsAbortController?.abort();
        prefetchAbortController?.abort();
        if (prefetchTimeout) {
            clearTimeout(prefetchTimeout);
            prefetchTimeout = null;
        }
    };

    return {
        // Search state
        searchQuery,
        isSearching,
        searchResults,
        searchError,

        // Details state
        selectedItem,
        isFetchingDetails,
        itemDetails,
        detailsError,

        // Prefetch state
        prefetchingId,

        // Actions
        search,
        fetchDetails,
        handleHover,
        handleHoverLeave,
        goBack,
        reset,
        cleanup,
    };
};
