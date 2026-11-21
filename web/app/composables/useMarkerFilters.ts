import type { SceneFilterOptions } from '~/types/scene';
import type { SavedMarkerFilterValues } from '~/types/marker';

export interface MarkerFilterState {
    rangesOnly: boolean;
    labels: string[];
    // Marker tags (all required)
    markerTags: string[];
    // Scene tags (all required)
    tags: string[];
    actors: string[];
    studio: string;
}

// Filter state key -> URL query key
const LIST_KEYS = ['labels', 'markerTags', 'tags', 'actors'] as const;
const URL_KEYS: Record<(typeof LIST_KEYS)[number], string> = {
    labels: 'labels',
    markerTags: 'marker_tags',
    tags: 'tags',
    actors: 'actors',
};

/**
 * Advanced filters of the marker pages (setting "Advanced Marker Filters"), synced with the URL.
 * Also keeps the seed of the "random" sort (in the URL) so that the pages of a random listing
 * stay consistent, and reshuffle() draws a new one.
 *
 * @param sortBy - the page sort ref (a new seed is drawn each time "random" is selected)
 * @param withLabels - whether the marker label filter applies (not on a single-label page)
 */
export function useMarkerFilters(sortBy: Ref<string>, withLabels = true) {
    const route = useRoute();
    const patchQuery = useUrlQueryPatch();
    const settingsStore = useSettingsStore();
    const { fetchFilterOptions } = useApiScenes();

    const enabled = computed(() => settingsStore.advancedMarkerFilters);

    const readList = (key: string): string[] => {
        const raw = route.query[key];
        return typeof raw === 'string' && raw ? raw.split(',').filter(Boolean) : [];
    };

    const state = reactive<MarkerFilterState>({
        rangesOnly: route.query.ranges === '1',
        labels: withLabels ? readList('labels') : [],
        markerTags: readList('marker_tags'),
        tags: readList('tags'),
        actors: readList('actors'),
        studio: typeof route.query.studio === 'string' ? route.query.studio : '',
    });

    const newSeed = newRandomSeed;
    // Random sort seed, kept in the URL (`seed`) so that a reload keeps the same order
    const urlSeed = Number(route.query.seed);
    const seed = ref(Number.isInteger(urlSeed) && urlSeed > 0 ? urlSeed : newSeed());

    /** Draws a new random order (the listing reloads through the params watcher). */
    const reshuffle = () => {
        seed.value = newSeed();
    };

    const activeCount = computed(
        () =>
            (state.rangesOnly ? 1 : 0) +
            state.labels.length +
            state.markerTags.length +
            state.tags.length +
            state.actors.length +
            (state.studio ? 1 : 0),
    );

    /** Query parameters for the marker listing API (filters only when the setting is on). */
    const params = computed<Record<string, string>>(() => {
        const p: Record<string, string> = {};
        if (sortBy.value === 'random') p.seed = String(seed.value);
        if (!enabled.value) return p;
        if (state.rangesOnly) p.ranges_only = 'true';
        if (withLabels && state.labels.length) p.labels = state.labels.join(',');
        if (state.markerTags.length) p.marker_tags = state.markerTags.join(',');
        if (state.tags.length) p.tags = state.tags.join(',');
        if (state.actors.length) p.actors = state.actors.join(',');
        if (state.studio) p.studio = state.studio;
        return p;
    });

    const reset = () => {
        state.rangesOnly = false;
        state.labels = [];
        state.markerTags = [];
        state.tags = [];
        state.actors = [];
        state.studio = '';
    };

    /** Current filters, as stored by a saved marker filter (sort/view added by the page). */
    const snapshot = (): SavedMarkerFilterValues => ({
        labels: withLabels ? [...state.labels] : [],
        marker_tags: [...state.markerTags],
        tags: [...state.tags],
        actors: [...state.actors],
        studio: state.studio,
        ranges_only: state.rangesOnly,
    });

    /** Applies the filters of a saved marker filter (sort/view are applied by the page). */
    const applySaved = (values: SavedMarkerFilterValues) => {
        state.rangesOnly = !!values.ranges_only;
        state.labels = withLabels ? [...(values.labels ?? [])] : [];
        state.markerTags = [...(values.marker_tags ?? [])];
        state.tags = [...(values.tags ?? [])];
        state.actors = [...(values.actors ?? [])];
        state.studio = values.studio ?? '';
    };

    // Keep the URL in sync so that a filtered view survives Back, a refresh or can be shared.
    // The writes are merged with the concurrent sort/pagination URL updates.
    watch(
        () => ({ ...state }),
        () => {
            const values: Record<string, string | undefined> = {
                ranges: state.rangesOnly ? '1' : undefined,
                studio: state.studio || undefined,
            };
            for (const key of LIST_KEYS) values[URL_KEYS[key]] = state[key].join(',');
            patchQuery(values);
        },
        { deep: true },
    );

    watch(sortBy, (value, previous) => {
        if (value === 'random' && previous !== 'random') seed.value = newSeed();
    });

    // Seed in the URL only while the random sort is active
    watch(
        [sortBy, seed],
        () => patchQuery({ seed: sortBy.value === 'random' ? String(seed.value) : undefined }),
        { immediate: true },
    );

    // Filter choices (scene tags, actors, studios, marker labels), loaded on first use
    const options = ref<SceneFilterOptions | null>(null);
    const loadOptions = async () => {
        if (options.value) return;
        try {
            options.value = await fetchFilterOptions();
        } catch {
            options.value = null;
        }
    };

    return {
        enabled,
        state,
        params,
        activeCount,
        reset,
        snapshot,
        applySaved,
        options,
        loadOptions,
        seed,
        reshuffle,
    };
}
