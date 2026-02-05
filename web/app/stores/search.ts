import type { SceneListItem, SceneFilterOptions } from '~/types/scene';
import type { SavedSearchFilters } from '~/types/saved_search';

export const useSearchStore = defineStore('search', () => {
    const api = useApi();
    const settingsStore = useSettingsStore();

    // Filter state
    const query = ref('');
    const selectedTags = ref<string[]>([]);
    const selectedActors = ref<string[]>([]);
    const studio = ref('');
    const minDuration = ref(0);
    const maxDuration = ref(0);
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

    // Filter options
    const filterOptions = ref<SceneFilterOptions>({
        studios: [],
        actors: [],
        tags: [],
        marker_labels: [],
        origins: [],
        types: [],
    });

    const hasActiveFilters = computed(() => {
        return (
            query.value !== '' ||
            selectedTags.value.length > 0 ||
            selectedActors.value.length > 0 ||
            studio.value !== '' ||
            minDuration.value > 0 ||
            maxDuration.value > 0 ||
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

    const search = async () => {
        isLoading.value = true;
        error.value = '';

        try {
            const params: Record<string, string | number | undefined> = {
                page: page.value,
                limit: limit.value,
            };

            if (query.value) params.q = query.value;
            if (selectedTags.value.length > 0) params.tags = selectedTags.value.join(',');
            if (selectedActors.value.length > 0) params.actors = selectedActors.value.join(',');
            if (studio.value) params.studio = studio.value;
            if (minDuration.value > 0) params.min_duration = minDuration.value;
            if (maxDuration.value > 0) params.max_duration = maxDuration.value;
            if (minDate.value) params.min_date = minDate.value;
            if (maxDate.value) params.max_date = maxDate.value;
            if (resolution.value) params.resolution = resolution.value;
            if (sort.value) params.sort = sort.value;
            if (sort.value === 'random' && seed.value) params.seed = seed.value;
            if (liked.value) params.liked = 'true';
            if (minRating.value > 0) params.min_rating = minRating.value;
            if (maxRating.value > 0) params.max_rating = maxRating.value;
            if (minJizzCount.value > 0) params.min_jizz_count = minJizzCount.value;
            if (maxJizzCount.value > 0) params.max_jizz_count = maxJizzCount.value;
            if (selectedMarkerLabels.value.length > 0)
                params.marker_labels = selectedMarkerLabels.value.join(',');
            if (matchType.value !== 'broad') params.match_type = matchType.value;

            const result = await api.searchScenes(params);
            scenes.value = result.data;
            total.value = result.total;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Search failed';
        } finally {
            isLoading.value = false;
        }
    };

    const reshuffle = () => {
        seed.value = generateSeed();
        page.value = 1;
        search();
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

    const resetFilters = () => {
        query.value = '';
        selectedTags.value = [];
        selectedActors.value = [];
        studio.value = '';
        minDuration.value = 0;
        maxDuration.value = 0;
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

    return {
        query,
        selectedTags,
        selectedActors,
        studio,
        minDuration,
        maxDuration,
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
        error,
        filterOptions,
        hasActiveFilters,
        search,
        reshuffle,
        loadFilterOptions,
        resetFilters,
        getCurrentFilters,
        loadFilters,
    };
});
