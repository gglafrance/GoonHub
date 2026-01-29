import type { Video, VideoFilterOptions } from '~/types/video';
import type { SavedSearchFilters } from '~/types/saved_search';

export const useSearchStore = defineStore('search', () => {
    const api = useApi();

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
    const limit = ref(20);
    const matchType = ref<'broad' | 'strict' | 'frequency'>('broad');

    // User interaction filters
    const liked = ref(false);
    const minRating = ref(0);
    const maxRating = ref(0);
    const minJizzCount = ref(0);
    const maxJizzCount = ref(0);

    // Results state
    const videos = ref<Video[]>([]);
    const total = ref(0);
    const isLoading = ref(false);
    const error = ref('');

    // Filter options
    const filterOptions = ref<VideoFilterOptions>({
        studios: [],
        actors: [],
        tags: [],
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
            matchType.value !== 'broad'
        );
    });

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
            if (liked.value) params.liked = 'true';
            if (minRating.value > 0) params.min_rating = minRating.value;
            if (maxRating.value > 0) params.max_rating = maxRating.value;
            if (minJizzCount.value > 0) params.min_jizz_count = minJizzCount.value;
            if (maxJizzCount.value > 0) params.max_jizz_count = maxJizzCount.value;
            if (matchType.value !== 'broad') params.match_type = matchType.value;

            const result = await api.searchVideos(params);
            videos.value = result.data;
            total.value = result.total;
        } catch (e: any) {
            error.value = e.message || 'Search failed';
        } finally {
            isLoading.value = false;
        }
    };

    const loadFilterOptions = async () => {
        try {
            const result = await api.fetchFilterOptions();
            filterOptions.value = {
                studios: result.studios || [],
                actors: result.actors || [],
                tags: result.tags || [],
            };
        } catch (e: any) {
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
        page.value = 1;
        liked.value = false;
        minRating.value = 0;
        maxRating.value = 0;
        minJizzCount.value = 0;
        maxJizzCount.value = 0;
        matchType.value = 'broad';
    };

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
        sort.value = filters.sort || '';
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
        page,
        limit,
        liked,
        minRating,
        maxRating,
        minJizzCount,
        maxJizzCount,
        matchType,
        videos,
        total,
        isLoading,
        error,
        filterOptions,
        hasActiveFilters,
        search,
        loadFilterOptions,
        resetFilters,
        getCurrentFilters,
        loadFilters,
    };
});
