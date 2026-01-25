import type { Video, VideoFilterOptions } from '~/types/video';

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
            maxJizzCount.value > 0
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
        videos,
        total,
        isLoading,
        error,
        filterOptions,
        hasActiveFilters,
        search,
        loadFilterOptions,
        resetFilters,
    };
});
