import type {
    SceneListItem,
    SceneFilterOptions,
    SceneListResponse,
    SourceSiteOption,
} from '~/types/scene';
import type { SavedSearchFilters } from '~/types/saved_search';
import type { LoadedPages, PageLoadPlan } from '~/utils/infinitePages';
import type { SearchUrlState } from '~/utils/searchUrlState';

// Library status filters: null = any, true = yes, false = no
export const SEARCH_STATUS_KEYS = [
    'watched',
    'rated',
    'has_markers',
    'has_actors',
    'has_tags',
    'has_studio',
    'has_source',
] as const;
export type SearchStatusKey = (typeof SEARCH_STATUS_KEYS)[number];
export type SearchStatus = Record<SearchStatusKey, boolean | null>;

const emptyStatus = (): SearchStatus => ({
    watched: null,
    rated: null,
    has_markers: null,
    has_actors: null,
    has_tags: null,
    has_studio: null,
    has_source: null,
});

/**
 * State and actions of a scene search (filters, sort, results, infinite scroll pages). Shared by
 * two store instances: the search page (`useSearchStore`, synced with the URL by the page) and
 * the search picker modal (`usePickerSearchStore`, never in the URL), so that opening the picker
 * does not change the search page. The search components read the instance of their context
 * (`useSearchContext`, see composables/useSearchContext.ts).
 */
function searchStoreSetup() {
    const api = useApi();
    const settingsStore = useSettingsStore();
    const perms = useMyPermissions();

    // Filter state
    const query = ref('');
    const selectedTags = ref<string[]>([]);
    const selectedActors = ref<string[]>([]);
    const studio = ref('');
    const minDuration = ref(0);
    const maxDuration = ref(0);
    // File size bounds in megabytes (0 = no bound)
    const minSize = ref(0);
    const maxSize = ref(0);
    const status = ref<SearchStatus>(emptyStatus());
    // Source sites (OR), normalized like the server does ("pornhub.com")
    const selectedSourceSites = ref<string[]>([]);
    const activeStatusCount = computed(
        () => SEARCH_STATUS_KEYS.filter((k) => status.value[k] !== null).length,
    );
    const statusParams = (): Record<string, string> => {
        const out: Record<string, string> = {};
        for (const k of SEARCH_STATUS_KEYS) {
            const v = status.value[k];
            if (v !== null) out[k] = String(v);
        }
        return out;
    };
    const minDate = ref('');
    const maxDate = ref('');
    const resolution = ref('');
    const sort = ref('');
    const page = ref(1);
    const limit = computed(() => settingsStore.videosPerPage);
    const matchType = ref<'broad' | 'strict' | 'frequency'>('broad');

    // Random sort seed
    const seed = ref(0);

    // User interaction filters
    const liked = ref(false);
    const minRating = ref(0);
    const maxRating = ref(0);
    const minJizzCount = ref(0);
    const maxJizzCount = ref(0);
    const selectedMarkerLabels = ref<string[]>([]);

    // Results state
    const scenes = ref<SceneListItem[]>([]);
    const total = ref(0);
    const isLoading = ref(false);
    const error = ref('');

    // Interaction sidecar maps from card_fields
    const ratings = ref<Record<string, number>>({});
    const likes = ref<Record<string, boolean>>({});
    const jizzCounts = ref<Record<string, number>>({});
    const watched = ref<Record<string, boolean>>({});

    // Filter options
    const filterOptions = ref<SceneFilterOptions>({
        studios: [],
        actors: [],
        tags: [],
        marker_labels: [],
        origins: [],
        types: [],
    });

    // Sites scenes come from, with their scene counts (for the Source filter)
    const sourceSites = ref<SourceSiteOption[]>([]);

    const hasActiveFilters = computed(() => {
        return (
            query.value !== '' ||
            selectedTags.value.length > 0 ||
            selectedActors.value.length > 0 ||
            studio.value !== '' ||
            minDuration.value > 0 ||
            maxDuration.value > 0 ||
            minSize.value > 0 ||
            maxSize.value > 0 ||
            activeStatusCount.value > 0 ||
            selectedSourceSites.value.length > 0 ||
            minDate.value !== '' ||
            maxDate.value !== '' ||
            resolution.value !== '' ||
            liked.value ||
            minRating.value > 0 ||
            maxRating.value > 0 ||
            minJizzCount.value > 0 ||
            maxJizzCount.value > 0 ||
            selectedMarkerLabels.value.length > 0 ||
            matchType.value !== 'broad'
        );
    });

    const generateSeed = () => Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);

    // Infinite scroll: the pages in the list and their filters (see utils/infinitePages)
    const loadedPages = ref<LoadedPages>({ first: 0, last: 0 });
    const isLoadingMore = ref(false);
    let searchRequest = 0;

    const mergeMaps = <T>(
        current: Record<string, T>,
        maps: (Record<string, T> | undefined)[],
        partial: boolean,
    ): Record<string, T> =>
        Object.assign(partial ? { ...current } : {}, ...maps.map((m) => m ?? {}));

    // Data version the list was (re)built at: after an edit anywhere (version changed) the list
    // held is not shown again on Back, it is loaded like a new one (see useDataVersion)
    let loadedVersion = -1;

    /** Runs a page plan; silent: refreshes the list in place, without the loading states. */
    const runSearch = async (plan: PageLoadPlan, silent = false) => {
        const request = ++searchRequest;
        const version = currentDataVersion();
        const partial = plan.mode !== 'replace';
        if (!silent) {
            if (partial) isLoadingMore.value = true;
            else isLoading.value = true;
        }
        error.value = '';

        try {
            const base = getSearchParams();
            const results: SceneListResponse[] = await Promise.all(
                plan.pages.map((p) => api.searchScenes({ ...base, page: p, limit: limit.value })),
            );
            // A newer search replaced this one
            if (request !== searchRequest) return;
            scenes.value = mergePageItems(
                scenes.value,
                results.flatMap((r) => r.data),
                plan.mode,
            );
            total.value = results[results.length - 1]?.total ?? 0;
            ratings.value = mergeMaps(
                ratings.value,
                results.map((r) => r.ratings),
                partial,
            );
            likes.value = mergeMaps(
                likes.value,
                results.map((r) => r.likes),
                partial,
            );
            jizzCounts.value = mergeMaps(
                jizzCounts.value,
                results.map((r) => r.jizz_counts),
                partial,
            );
            watched.value = mergeMaps(
                watched.value,
                results.map((r) => r.watched),
                partial,
            );
            loadedPages.value = commitPageLoad(loadedPages.value, plan);
            // Appended/prepended pages keep the version of the list they join
            if (!partial) loadedVersion = version;
        } catch (e: unknown) {
            if (request === searchRequest) {
                error.value = e instanceof Error ? e.message : 'Search failed';
                loadedVersion = -1;
            }
        } finally {
            if (request === searchRequest) {
                isLoading.value = false;
                isLoadingMore.value = false;
            }
        }
    };

    /**
     * Loads the current page (appended to the list when it is the next one with infinite scroll).
     * Coming back to the list already held (Back from a scene: same filters, same URL page, no
     * edit since), it stays on screen at once, with all its infinite scroll pages, and is
     * refreshed in place instead of flashing a spinner.
     */
    const search = () => {
        const context = JSON.stringify({ ...getSearchParams(), limit: limit.value });
        const infinite = settingsStore.infiniteScroll;
        const held = loadedPages.value;
        const isHeld =
            held.last > 0 &&
            held.last === page.value &&
            (held.context ?? '') === context &&
            loadedVersion === currentDataVersion() &&
            (infinite || held.first === held.last);
        if (isHeld) {
            const pages: number[] = [];
            for (let p = held.first; p <= held.last; p++) pages.push(p);
            // A long scrolled list is kept as is (no edit since): refreshing it would be heavy
            if (pages.length > INFINITE_REBUILD_PAGES) return Promise.resolve();
            return runSearch({ pages, mode: 'replace', context }, true);
        }
        return runSearch(planPageLoad(held, page.value, infinite, context));
    };

    // Infinite scroll opened on a later page: loads the page before the list
    const loadPrevious = async () => {
        const plan = planPreviousPage(loadedPages.value);
        if (plan) await runSearch(plan);
    };

    const reshuffle = () => {
        seed.value = generateSeed();
        page.value = 1;
    };

    const loadFilterOptions = async () => {
        try {
            const result = await api.fetchFilterOptions();
            filterOptions.value = {
                studios: result.studios || [],
                actors: result.actors || [],
                tags: result.tags || [],
                marker_labels: result.marker_labels || [],
                origins: result.origins || [],
                types: result.types || [],
            };
        } catch (e: unknown) {
            console.error('Failed to load filter options:', e);
        }
    };

    const loadSourceSites = async () => {
        try {
            const result = await api.fetchSourceSites();
            sourceSites.value = result.sites || [];
        } catch (e: unknown) {
            console.error('Failed to load source sites:', e);
        }
    };

    const resetFilters = () => {
        query.value = '';
        selectedTags.value = [];
        selectedActors.value = [];
        studio.value = '';
        minDuration.value = 0;
        maxDuration.value = 0;
        minSize.value = 0;
        maxSize.value = 0;
        status.value = emptyStatus();
        selectedSourceSites.value = [];
        minDate.value = '';
        maxDate.value = '';
        resolution.value = '';
        sort.value = '';
        seed.value = 0;
        page.value = 1;
        liked.value = false;
        minRating.value = 0;
        maxRating.value = 0;
        minJizzCount.value = 0;
        maxJizzCount.value = 0;
        selectedMarkerLabels.value = [];
        matchType.value = 'broad';
    };

    // Generate seed when switching to random, clear when switching away
    watch(sort, (newSort) => {
        if (newSort === 'random' && seed.value === 0) {
            seed.value = generateSeed();
        } else if (newSort !== 'random') {
            seed.value = 0;
        }
    });

    // Export current filters as SavedSearchFilters object (omit pagination)
    const getCurrentFilters = (): SavedSearchFilters => {
        const filters: SavedSearchFilters = {};

        if (query.value) filters.query = query.value;
        if (matchType.value !== 'broad') filters.match_type = matchType.value;
        if (selectedTags.value.length > 0) filters.selected_tags = [...selectedTags.value];
        if (selectedActors.value.length > 0) filters.selected_actors = [...selectedActors.value];
        if (studio.value) filters.studio = studio.value;
        if (resolution.value) filters.resolution = resolution.value;
        if (minDuration.value > 0) filters.min_duration = minDuration.value;
        if (maxDuration.value > 0) filters.max_duration = maxDuration.value;
        if (minSize.value > 0) filters.min_size = minSize.value;
        if (maxSize.value > 0) filters.max_size = maxSize.value;
        for (const k of SEARCH_STATUS_KEYS) {
            const v = status.value[k];
            if (v !== null) filters[k] = v;
        }
        if (selectedSourceSites.value.length > 0)
            filters.source_sites = [...selectedSourceSites.value];
        if (minDate.value) filters.min_date = minDate.value;
        if (maxDate.value) filters.max_date = maxDate.value;
        if (liked.value) filters.liked = true;
        if (minRating.value > 0) filters.min_rating = minRating.value;
        if (maxRating.value > 0) filters.max_rating = maxRating.value;
        if (minJizzCount.value > 0) filters.min_jizz_count = minJizzCount.value;
        if (maxJizzCount.value > 0) filters.max_jizz_count = maxJizzCount.value;
        if (selectedMarkerLabels.value.length > 0)
            filters.selected_marker_labels = [...selectedMarkerLabels.value];
        if (sort.value) filters.sort = sort.value;

        return filters;
    };

    // Load filters from a SavedSearchFilters object
    const loadFilters = (filters: SavedSearchFilters) => {
        query.value = filters.query || '';
        matchType.value = (filters.match_type as 'broad' | 'strict' | 'frequency') || 'broad';
        selectedTags.value = filters.selected_tags || [];
        selectedActors.value = filters.selected_actors || [];
        studio.value = filters.studio || '';
        resolution.value = filters.resolution || '';
        minDuration.value = filters.min_duration || 0;
        maxDuration.value = filters.max_duration || 0;
        minSize.value = filters.min_size || 0;
        maxSize.value = filters.max_size || 0;
        status.value = emptyStatus();
        for (const k of SEARCH_STATUS_KEYS) {
            const v = filters[k];
            if (typeof v === 'boolean') status.value[k] = v;
        }
        selectedSourceSites.value = filters.source_sites || [];
        minDate.value = filters.min_date || '';
        maxDate.value = filters.max_date || '';
        liked.value = filters.liked || false;
        minRating.value = filters.min_rating || 0;
        maxRating.value = filters.max_rating || 0;
        minJizzCount.value = filters.min_jizz_count || 0;
        maxJizzCount.value = filters.max_jizz_count || 0;
        selectedMarkerLabels.value = filters.selected_marker_labels || [];
        sort.value = filters.sort || '';
        seed.value = filters.sort === 'random' ? generateSeed() : 0;
        page.value = 1; // Reset pagination when loading filters
    };

    const getSearchParams = (): Record<string, string | number | undefined> => {
        const params: Record<string, string | number | undefined> = {};
        if (query.value) params.q = query.value;
        if (selectedTags.value.length > 0) params.tags = selectedTags.value.join(',');
        if (selectedActors.value.length > 0) params.actors = selectedActors.value.join(',');
        if (studio.value) params.studio = studio.value;
        if (minDuration.value > 0) params.min_duration = minDuration.value;
        if (maxDuration.value > 0) params.max_duration = maxDuration.value;
        // Size filters and sorts need files:view_size (the API refuses them otherwise): a
        // size filter or sort from an old URL or saved search is left out
        const canSize = perms.has('files:view_size');
        if (canSize && minSize.value > 0) params.min_size = minSize.value;
        if (canSize && maxSize.value > 0) params.max_size = maxSize.value;
        Object.assign(params, statusParams());
        if (selectedSourceSites.value.length > 0)
            params.source_site = selectedSourceSites.value.join(',');
        if (minDate.value) params.min_date = minDate.value;
        if (maxDate.value) params.max_date = maxDate.value;
        if (resolution.value) params.resolution = resolution.value;
        if (sort.value && sceneSortAllowed(sort.value, perms.has)) params.sort = sort.value;
        if (sort.value === 'random' && seed.value) params.seed = seed.value;
        if (liked.value) params.liked = 'true';
        if (minRating.value > 0) params.min_rating = minRating.value;
        if (maxRating.value > 0) params.max_rating = maxRating.value;
        if (minJizzCount.value > 0) params.min_jizz_count = minJizzCount.value;
        if (maxJizzCount.value > 0) params.max_jizz_count = maxJizzCount.value;
        if (selectedMarkerLabels.value.length > 0)
            params.marker_labels = selectedMarkerLabels.value.join(',');
        if (matchType.value !== 'broad') params.match_type = matchType.value;
        return params;
    };

    /** Search state kept in the URL (see utils/searchUrlState.ts). */
    const getUrlState = (): SearchUrlState => ({
        query: query.value,
        selectedTags: selectedTags.value,
        selectedActors: selectedActors.value,
        studio: studio.value,
        minDuration: minDuration.value,
        maxDuration: maxDuration.value,
        minSize: minSize.value,
        maxSize: maxSize.value,
        status: status.value,
        selectedSourceSites: selectedSourceSites.value,
        minDate: minDate.value,
        maxDate: maxDate.value,
        resolution: resolution.value,
        sort: sort.value,
        seed: seed.value,
        page: page.value,
        liked: liked.value,
        minRating: minRating.value,
        maxRating: maxRating.value,
        minJizzCount: minJizzCount.value,
        maxJizzCount: maxJizzCount.value,
        selectedMarkerLabels: selectedMarkerLabels.value,
        matchType: matchType.value,
    });

    /**
     * Applies the state read from the URL. Returns false when the URL had to be completed (a
     * random sort without its seed gets one, so that Back brings the same order back).
     */
    const applyUrlState = (s: SearchUrlState): boolean => {
        query.value = s.query;
        selectedTags.value = s.selectedTags;
        selectedActors.value = s.selectedActors;
        studio.value = s.studio;
        minDuration.value = s.minDuration;
        maxDuration.value = s.maxDuration;
        minSize.value = s.minSize;
        maxSize.value = s.maxSize;
        status.value = { ...s.status };
        selectedSourceSites.value = s.selectedSourceSites;
        minDate.value = s.minDate;
        maxDate.value = s.maxDate;
        resolution.value = s.resolution;
        sort.value = s.sort;
        page.value = s.page;
        liked.value = s.liked;
        minRating.value = s.minRating;
        maxRating.value = s.maxRating;
        minJizzCount.value = s.minJizzCount;
        maxJizzCount.value = s.maxJizzCount;
        selectedMarkerLabels.value = s.selectedMarkerLabels;
        matchType.value = s.matchType;
        if (s.sort === 'random' && !s.seed) {
            seed.value = generateSeed();
            return false;
        }
        seed.value = s.seed;
        return true;
    };

    return {
        query,
        selectedTags,
        selectedActors,
        studio,
        minDuration,
        maxDuration,
        minSize,
        maxSize,
        status,
        activeStatusCount,
        selectedSourceSites,
        sourceSites,
        minDate,
        maxDate,
        resolution,
        sort,
        seed,
        page,
        limit,
        liked,
        minRating,
        maxRating,
        minJizzCount,
        maxJizzCount,
        selectedMarkerLabels,
        matchType,
        scenes,
        total,
        isLoading,
        isLoadingMore,
        loadedPages,
        error,
        ratings,
        likes,
        jizzCounts,
        watched,
        filterOptions,
        hasActiveFilters,
        getSearchParams,
        getUrlState,
        applyUrlState,
        search,
        loadPrevious,
        reshuffle,
        loadFilterOptions,
        loadSourceSites,
        resetFilters,
        getCurrentFilters,
        loadFilters,
    };
}

export const useSearchStore = defineStore('search', searchStoreSetup);

/** Search of the picker modal (components/picker): same state as the search page, isolated. */
export const usePickerSearchStore = defineStore('search-picker', searchStoreSetup);

/** Criteria editor of the profile custom lists (components/profile/edit): isolated too. */
export const useProfileListSearchStore = defineStore('search-profile-list', searchStoreSetup);

/** Any search store instance (they only differ by their id). */
export type SearchStore =
    | ReturnType<typeof useSearchStore>
    | ReturnType<typeof usePickerSearchStore>
    | ReturnType<typeof useProfileListSearchStore>;
