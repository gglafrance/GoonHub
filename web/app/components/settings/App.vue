<script setup lang="ts">
import type { SortOrder } from '~/types/settings';

const settingsStore = useSettingsStore();
const authStore = useAuthStore();
const { triggerReindex, getSearchConfig, updateSearchConfig } = useApiAdmin();
const { layout: keyboardLayout, setLayout: setKeyboardLayout } = useKeyboardLayout();

// Writable computeds on draft
const appVideosPerPage = computed({
    get: () => settingsStore.draft?.videos_per_page ?? 20,
    set: (v) => {
        if (settingsStore.draft) {
            settingsStore.draft.videos_per_page = Math.min(
                Math.max(1, v),
                settingsStore.maxItemsPerPage,
            );
        }
    },
});

const appSortOrder = computed({
    get: () => (settingsStore.draft?.default_sort_order ?? 'created_at_desc') as SortOrder,
    set: (v: SortOrder) => {
        if (settingsStore.draft) settingsStore.draft.default_sort_order = v;
    },
});

const appMarkerThumbnailCycling = computed({
    get: () => settingsStore.draft?.marker_thumbnail_cycling ?? true,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.marker_thumbnail_cycling = v;
    },
});

const appShowPageSizeSelector = computed({
    get: () => settingsStore.draft?.show_page_size_selector ?? false,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.show_page_size_selector = v;
    },
});

const sortActors = computed({
    get: () => settingsStore.draft?.sort_preferences?.actors ?? 'name_asc',
    set: (v) => {
        if (settingsStore.draft?.sort_preferences) settingsStore.draft.sort_preferences.actors = v;
    },
});

const sortStudios = computed({
    get: () => settingsStore.draft?.sort_preferences?.studios ?? 'name_asc',
    set: (v) => {
        if (settingsStore.draft?.sort_preferences) settingsStore.draft.sort_preferences.studios = v;
    },
});

const sortMarkers = computed({
    get: () => settingsStore.draft?.sort_preferences?.markers ?? 'label_asc',
    set: (v) => {
        if (settingsStore.draft?.sort_preferences) settingsStore.draft.sort_preferences.markers = v;
    },
});

const sortActorScenes = computed({
    get: () => settingsStore.draft?.sort_preferences?.actor_scenes ?? '',
    set: (v) => {
        if (settingsStore.draft?.sort_preferences)
            settingsStore.draft.sort_preferences.actor_scenes = v;
    },
});

const sortStudioScenes = computed({
    get: () => settingsStore.draft?.sort_preferences?.studio_scenes ?? '',
    set: (v) => {
        if (settingsStore.draft?.sort_preferences)
            settingsStore.draft.sort_preferences.studio_scenes = v;
    },
});

// Search index state
const isReindexing = ref(false);
const reindexMessage = ref('');
const reindexError = ref('');

// Search config state (admin only)
const isAdmin = computed(() => authStore.user?.role === 'admin');
const maxTotalHits = ref(100000);
const isSavingSearchConfig = ref(false);
const searchConfigMessage = ref('');
const searchConfigError = ref('');

const sortOptions: { value: SortOrder; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest First' },
    { value: 'duration_desc', label: 'Longest First' },
    { value: 'size_asc', label: 'Smallest First' },
    { value: 'size_desc', label: 'Largest First' },
    { value: 'random', label: 'Random' },
];

const actorSortOptions = [
    { value: 'name_asc', label: 'Name A-Z' },
    { value: 'name_desc', label: 'Name Z-A' },
    { value: 'scene_count_desc', label: 'Most Scenes' },
    { value: 'scene_count_asc', label: 'Least Scenes' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
];

const studioSortOptions = [
    { value: 'name_asc', label: 'Name A-Z' },
    { value: 'name_desc', label: 'Name Z-A' },
    { value: 'scene_count_desc', label: 'Most Scenes' },
    { value: 'scene_count_asc', label: 'Least Scenes' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
];

const markerSortOptions = [
    { value: 'label_asc', label: 'A-Z' },
    { value: 'label_desc', label: 'Z-A' },
    { value: 'count_desc', label: 'Most Markers' },
    { value: 'count_asc', label: 'Fewest Markers' },
    { value: 'recent', label: 'Recently Added' },
    { value: 'oldest', label: 'Oldest' },
];

const entitySceneSortOptions = [
    { value: '', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
    { value: 'random', label: 'Random' },
];

const loadSearchConfig = async () => {
    if (!isAdmin.value) return;
    try {
        const data = await getSearchConfig();
        maxTotalHits.value = data.max_total_hits;
    } catch {
        // Silently fail - default value is already set
    }
};

onMounted(() => {
    loadSearchConfig();
});

const handleReindex = async () => {
    reindexMessage.value = '';
    reindexError.value = '';
    isReindexing.value = true;
    try {
        await triggerReindex();
        reindexMessage.value = 'Search index rebuild started. This may take a few moments.';
    } catch (e: unknown) {
        reindexError.value = e instanceof Error ? e.message : 'Failed to trigger reindex';
    } finally {
        isReindexing.value = false;
    }
};

const handleSaveSearchConfig = async () => {
    searchConfigMessage.value = '';
    searchConfigError.value = '';
    isSavingSearchConfig.value = true;
    try {
        await updateSearchConfig({ max_total_hits: maxTotalHits.value });
        searchConfigMessage.value = 'Search configuration saved';
    } catch (e: unknown) {
        searchConfigError.value = e instanceof Error ? e.message : 'Failed to save search config';
    } finally {
        isSavingSearchConfig.value = false;
    }
};
</script>

<template>
    <div class="space-y-6">
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">App Preferences</h3>
            <div class="space-y-5">
                <!-- Videos Per Page -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Videos Per Page
                    </label>
                    <input
                        v-model.number="appVideosPerPage"
                        type="number"
                        min="1"
                        :max="settingsStore.maxItemsPerPage"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full max-w-32 rounded-lg border px-3.5 py-2.5 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                    <p class="text-dim mt-1.5 text-[11px]">
                        Value between 1 and {{ settingsStore.maxItemsPerPage }}
                    </p>
                </div>

                <!-- Page Size Selector -->
                <div class="flex items-center justify-between">
                    <div>
                        <label class="text-sm font-medium text-white"> Page Size Selector </label>
                        <p class="text-dim mt-0.5 text-xs">
                            Show a page size dropdown on paginated pages
                        </p>
                    </div>
                    <UiToggle v-model="appShowPageSizeSelector" />
                </div>

                <!-- Sort Order -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Default Sort Order
                    </label>
                    <p class="text-dim mb-2 text-xs">
                        Default sort for the main scenes/search page
                    </p>
                    <UiSelectMenu v-model="appSortOrder" :options="sortOptions" class="max-w-64" />
                </div>

                <!-- Page Sort Preferences -->
                <div class="border-border border-t pt-5">
                    <h4 class="mb-4 text-xs font-semibold text-white">Page Sort Defaults</h4>
                    <p class="text-dim mb-4 text-xs">
                        Set the default sort order for each page. Used when no sort parameter is in
                        the URL.
                    </p>
                    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Actors
                            </label>
                            <UiSelectMenu
                                v-model="sortActors"
                                :options="actorSortOptions"
                                class="max-w-64"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Studios
                            </label>
                            <UiSelectMenu
                                v-model="sortStudios"
                                :options="studioSortOptions"
                                class="max-w-64"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Markers
                            </label>
                            <UiSelectMenu
                                v-model="sortMarkers"
                                :options="markerSortOptions"
                                class="max-w-64"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Actor Scenes
                            </label>
                            <UiSelectMenu
                                v-model="sortActorScenes"
                                :options="entitySceneSortOptions"
                                class="max-w-64"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Studio Scenes
                            </label>
                            <UiSelectMenu
                                v-model="sortStudioScenes"
                                :options="entitySceneSortOptions"
                                class="max-w-64"
                            />
                        </div>
                    </div>
                </div>

                <!-- Keyboard Layout -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Keyboard Layout
                    </label>
                    <p class="text-dim mb-3 text-xs">
                        Adjusts keyboard shortcuts for your keyboard layout
                    </p>
                    <div class="flex gap-2">
                        <button
                            :class="[
                                'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                                keyboardLayout === 'qwerty'
                                    ? 'border-lava bg-lava/10 text-lava'
                                    : `border-border hover:border-border-hover text-muted
                                        hover:text-white`,
                            ]"
                            @click="setKeyboardLayout('qwerty')"
                        >
                            QWERTY
                        </button>
                        <button
                            :class="[
                                'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                                keyboardLayout === 'azerty'
                                    ? 'border-lava bg-lava/10 text-lava'
                                    : `border-border hover:border-border-hover text-muted
                                        hover:text-white`,
                            ]"
                            @click="setKeyboardLayout('azerty')"
                        >
                            AZERTY
                        </button>
                    </div>
                </div>

                <!-- Marker Thumbnail Cycling -->
                <div class="flex items-center justify-between">
                    <div>
                        <label class="text-sm font-medium text-white">
                            Marker Thumbnail Cycling
                        </label>
                        <p class="text-dim mt-0.5 text-xs">
                            Automatically cycle through thumbnails on marker label cards
                        </p>
                    </div>
                    <UiToggle v-model="appMarkerThumbnailCycling" />
                </div>
            </div>
        </div>

        <!-- Search Configuration (admin only) -->
        <div v-if="isAdmin" class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">Search</h3>
            <p class="text-dim mb-4 text-xs">Configure search engine settings.</p>

            <div
                v-if="searchConfigMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald mb-4 rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ searchConfigMessage }}
            </div>
            <div
                v-if="searchConfigError"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ searchConfigError }}
            </div>

            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Max Total Hits
                </label>
                <p class="text-dim mb-2 text-xs">
                    Maximum number of search results that can be counted. Increase this if your
                    library has more scenes than the current limit.
                </p>
                <div class="flex items-center gap-3">
                    <input
                        v-model.number="maxTotalHits"
                        type="number"
                        min="1000"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full max-w-48 rounded-lg border px-3.5 py-2.5 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                    <button
                        :disabled="isSavingSearchConfig"
                        class="border-border hover:border-lava/40 hover:bg-lava/10 rounded-lg border
                            px-4 py-2 text-xs font-medium text-white transition-all
                            disabled:cursor-not-allowed disabled:opacity-40"
                        @click="handleSaveSearchConfig"
                    >
                        {{ isSavingSearchConfig ? 'Saving...' : 'Save' }}
                    </button>
                </div>
            </div>
        </div>

        <!-- Search Index -->
        <div class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">Search Index</h3>
            <p class="text-dim mb-4 text-xs">
                Rebuild the search index to sync all video data including actors, tags, and view
                counts.
            </p>

            <div
                v-if="reindexMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald mb-4 rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ reindexMessage }}
            </div>
            <div
                v-if="reindexError"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ reindexError }}
            </div>

            <button
                :disabled="isReindexing"
                class="border-border hover:border-lava/40 hover:bg-lava/10 flex items-center gap-2
                    rounded-lg border px-4 py-2 text-xs font-medium text-white transition-all
                    disabled:cursor-not-allowed disabled:opacity-40"
                @click="handleReindex"
            >
                <Icon
                    name="heroicons:arrow-path"
                    size="14"
                    :class="{ 'animate-spin': isReindexing }"
                />
                {{ isReindexing ? 'Rebuilding...' : 'Rebuild Search Index' }}
            </button>
        </div>
    </div>
</template>
