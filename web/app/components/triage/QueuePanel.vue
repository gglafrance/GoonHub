<script setup lang="ts">
import type { SearchStatusKey } from '~/stores/search';
import type { SavedSearch } from '~/types/saved_search';

// Source of the triage queue: quick presets, a saved search, and the regular search filters
// (bound to the search store, like on the search page).
const searchStore = useSearchStore();
const { fetchSavedSearches } = useApiSavedSearches();

interface Preset {
    key: SearchStatusKey;
    value: boolean;
    label: string;
    icon: string;
}

const presets: Preset[] = [
    { key: 'has_tags', value: false, label: 'No tags', icon: 'heroicons:tag' },
    { key: 'has_actors', value: false, label: 'No actors', icon: 'heroicons:user' },
    { key: 'rated', value: false, label: 'Unrated', icon: 'heroicons:star' },
    { key: 'has_studio', value: false, label: 'No studio', icon: 'heroicons:building-office-2' },
    { key: 'watched', value: false, label: 'Never watched', icon: 'heroicons:eye-slash' },
];

const sortOptions = [
    { value: '', label: 'Default order' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'size_desc', label: 'Largest' },
    { value: 'view_count_desc', label: 'Most viewed' },
    { value: 'random', label: 'Random' },
];

// A preset is active when it is the only filter
const isPresetActive = (preset: Preset) => {
    const params = searchStore.getSearchParams();
    delete params.sort;
    delete params.seed;
    const keys = Object.keys(params);
    return keys.length === 1 && params[preset.key] === String(preset.value);
};

const applyPreset = (preset: Preset) => {
    const sort = searchStore.sort;
    searchStore.resetFilters();
    searchStore.sort = sort;
    searchStore.status[preset.key] = preset.value;
};

const savedSearches = ref<SavedSearch[]>([]);
const selectedSaved = ref('');

onMounted(async () => {
    try {
        const res = await fetchSavedSearches();
        savedSearches.value = res.data || [];
    } catch {
        savedSearches.value = [];
    }
});

const loadSaved = () => {
    const search = savedSearches.value.find((s) => s.uuid === selectedSaved.value);
    if (search) searchStore.loadFilters(search.filters);
    selectedSaved.value = '';
};
</script>

<template>
    <div class="space-y-4">
        <div>
            <h3 class="mb-2 text-xs font-semibold tracking-wide text-white uppercase">Queue</h3>
            <div class="flex flex-wrap gap-1.5">
                <button
                    v-for="preset in presets"
                    :key="preset.key"
                    class="flex items-center gap-1 rounded-full border px-2.5 py-1 text-[11px]
                        font-medium transition-all"
                    :class="
                        isPresetActive(preset)
                            ? 'border-lava/50 bg-lava/15 text-lava'
                            : 'border-border text-muted hover:border-lava/30 hover:text-white'
                    "
                    @click="applyPreset(preset)"
                >
                    <Icon :name="preset.icon" size="11" />
                    {{ preset.label }}
                </button>
            </div>
        </div>

        <div v-if="savedSearches.length > 0" class="relative">
            <select
                v-model="selectedSaved"
                class="border-border bg-panel focus:border-lava/50 h-8 w-full cursor-pointer
                    appearance-none rounded-lg border pr-8 pl-3 text-xs text-white
                    focus:outline-none"
                @change="loadSaved"
            >
                <option value="">Load a saved search...</option>
                <option v-for="s in savedSearches" :key="s.uuid" :value="s.uuid">
                    {{ s.name }}
                </option>
            </select>
            <Icon
                name="heroicons:bookmark"
                size="12"
                class="text-dim pointer-events-none absolute top-1/2 right-3 -translate-y-1/2"
            />
        </div>

        <div class="space-y-2">
            <div class="relative">
                <Icon
                    name="heroicons:magnifying-glass"
                    size="14"
                    class="text-dim absolute top-1/2 left-2.5 -translate-y-1/2"
                />
                <input
                    v-model.lazy="searchStore.query"
                    type="text"
                    placeholder="Search text (Enter)"
                    class="border-border bg-surface placeholder:text-dim h-8 w-full rounded-lg
                        border pr-3 pl-8 text-xs text-white transition-colors focus:border-white/20
                        focus:outline-none"
                    @keydown.enter="($event.target as HTMLInputElement).blur()"
                />
            </div>
            <UiSortSelect v-model="searchStore.sort" :options="sortOptions" class="h-8" />
        </div>

        <SearchFilters />
    </div>
</template>
