<script setup lang="ts">
import type { SortOrder } from '~/types/settings';

const settingsStore = useSettingsStore();

const appSortOrder = computed({
    get: () => (settingsStore.draft?.default_sort_order ?? 'created_at_desc') as SortOrder,
    set: (v: SortOrder) => {
        if (settingsStore.draft) settingsStore.draft.default_sort_order = v;
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

const sceneSortOptions: { value: SortOrder; label: string }[] = [
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
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-4 text-sm font-semibold text-white">Page Sort Defaults</h3>
        <p class="text-dim mb-5 text-xs">
            Set the default sort order for each page. Used when no sort parameter is in the URL.
        </p>

        <!-- Default Sort Order -->
        <div class="mb-5">
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Default Sort Order
            </label>
            <p class="text-dim mb-2 text-xs">Default sort for the main scenes/search page</p>
            <UiSelectMenu v-model="appSortOrder" :options="sceneSortOptions" class="max-w-64" />
        </div>

        <div class="border-border border-t pt-5">
            <h4 class="mb-4 text-xs font-semibold text-white">Page Sort Defaults</h4>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Actors
                </label>
                <UiSelectMenu v-model="sortActors" :options="actorSortOptions" class="max-w-64" />
            </div>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Studios
                </label>
                <UiSelectMenu v-model="sortStudios" :options="studioSortOptions" class="max-w-64" />
            </div>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Markers
                </label>
                <UiSelectMenu v-model="sortMarkers" :options="markerSortOptions" class="max-w-64" />
            </div>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
</template>
