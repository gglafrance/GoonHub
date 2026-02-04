<script setup lang="ts">
import type { PornDBScene } from '~/types/porndb';
import type { ConfidenceBreakdown } from '~/types/bulk-match';

// Extended scene data for search with confidence calculation
interface SceneSearchInfo {
    title?: string;
    studio?: string | null;
    // Extended fields for confidence & parsing
    original_filename?: string;
    actors?: string[];
    duration?: number;
}

// Extended result type with confidence
interface SearchResultWithConfidence extends PornDBScene {
    confidence?: ConfidenceBreakdown;
}

const props = defineProps<{
    scene: SceneSearchInfo | null;
}>();

const emit = defineEmits<{
    select: [scene: PornDBScene];
}>();

const api = useApi();
const settingsStore = useSettingsStore();
const { calculateConfidence } = useConfidenceCalculator();
const { applyRules, getBuiltInPresets } = useParsingRulesEngine();

const title = ref('');
const year = ref('');
const site = ref('');
const selectedPresetId = ref<string | null>(null);

const searching = ref(false);
const hasSearched = ref(false);
const searchResults = ref<SearchResultWithConfidence[]>([]);
const searchError = ref('');
const loadingScene = ref(false);

// Available presets (built-in + user presets)
const availablePresets = computed(() => {
    const builtIn = getBuiltInPresets();
    const userPresets = settingsStore.parsingRules?.presets.filter((p) => !p.isBuiltIn) || [];
    return [...builtIn, ...userPresets];
});

// Get rules for selected preset
const selectedPresetRules = computed(() => {
    if (!selectedPresetId.value) return undefined;

    // Check built-in presets
    const builtIn = getBuiltInPresets().find((p) => p.id === selectedPresetId.value);
    if (builtIn) return builtIn.rules;

    // Check user presets
    const userPreset = settingsStore.parsingRules?.presets.find(
        (p) => p.id === selectedPresetId.value,
    );
    return userPreset?.rules;
});

// Check if scene has enough data for confidence calculation
const canCalculateConfidence = computed(() => {
    return (
        props.scene &&
        (props.scene.original_filename || props.scene.duration || props.scene.actors?.length)
    );
});

const hasAnyFilter = computed(() => {
    return title.value.trim() !== '' || year.value.trim() !== '' || site.value.trim() !== '';
});

function formatDuration(seconds: number): string {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function clearFilters() {
    title.value = '';
    year.value = '';
    site.value = '';
}

function getQueryFromScene(): string {
    if (!props.scene) return '';

    // If parsing rules are selected, apply them to original_filename
    if (selectedPresetRules.value && props.scene.original_filename) {
        return applyRules(props.scene.original_filename, selectedPresetRules.value);
    }

    // Fallback to title
    return props.scene.title || '';
}

function populateFromScene() {
    if (!props.scene) return;

    title.value = getQueryFromScene();

    if (props.scene.studio) {
        site.value = props.scene.studio;
    }
    const dateStr = props.scene.title?.match(/(?:^|_|\s)(19\d{2}|20\d{2})(?:_|\s|$)/)?.[1];
    if (dateStr) {
        year.value = dateStr;
    }
}

async function searchScenes() {
    if (!hasAnyFilter.value) return;

    searching.value = true;
    searchError.value = '';
    searchResults.value = [];
    hasSearched.value = true;

    try {
        const params: {
            q?: string;
            title?: string;
            year?: number;
            site?: string;
        } = {};

        if (title.value.trim()) {
            params.title = title.value.trim();
        }
        if (year.value.trim()) {
            const y = parseInt(year.value, 10);
            if (!isNaN(y) && y > 1900) {
                params.year = y;
            }
        }
        if (site.value.trim()) {
            params.site = site.value.trim();
        }

        const results = await api.searchPornDBScenes(params);

        // Calculate confidence for each result if we have scene data
        if (canCalculateConfidence.value && props.scene) {
            const localScene = {
                id: 0, // Not used for confidence
                title: props.scene.original_filename || props.scene.title || '',
                original_filename: props.scene.original_filename || '',
                porndb_scene_id: null,
                actors: props.scene.actors || [],
                studio: props.scene.studio || null,
                thumbnail_path: '',
                duration: props.scene.duration || 0,
            };

            const resultsWithConfidence: SearchResultWithConfidence[] = results.map(
                (result: PornDBScene) => ({
                    ...result,
                    confidence: calculateConfidence(localScene, result),
                }),
            );

            // Sort by confidence (highest first)
            resultsWithConfidence.sort(
                (a, b) => (b.confidence?.total || 0) - (a.confidence?.total || 0),
            );
            searchResults.value = resultsWithConfidence;
        } else {
            searchResults.value = results;
        }
    } catch (e: unknown) {
        searchError.value = e instanceof Error ? e.message : 'Search failed';
    } finally {
        searching.value = false;
    }
}

function getConfidenceColorClass(confidence: ConfidenceBreakdown | undefined): string {
    if (!confidence) return 'bg-white/10 text-white/60';
    if (confidence.total >= 80) return 'bg-emerald-500/15 text-emerald-400';
    if (confidence.total >= 50) return 'bg-amber-500/15 text-amber-400';
    return 'bg-red-500/15 text-red-400';
}

function getConfidenceTooltip(confidence: ConfidenceBreakdown | undefined): string {
    if (!confidence) return '';
    return `Title: ${confidence.titleScore}/30, Actors: ${confidence.actorScore}/30, Studio: ${confidence.studioScore}/20, Duration: ${confidence.durationScore}/20`;
}

// Re-populate search field when preset changes
function handlePresetChange() {
    title.value = getQueryFromScene();
}

async function selectScene(scene: PornDBScene) {
    loadingScene.value = true;

    try {
        const details = await api.getPornDBScene(scene.id);
        emit('select', details);
    } catch (e: unknown) {
        searchError.value = e instanceof Error ? e.message : 'Failed to load scene details';
    } finally {
        loadingScene.value = false;
    }
}

onMounted(async () => {
    // Load parsing rules if not already loaded
    if (!settingsStore.parsingRules) {
        await settingsStore.loadParsingRules();
    }
    // Set default preset from settings
    selectedPresetId.value = settingsStore.parsingRules?.activePresetId || null;

    populateFromScene();
});
</script>

<template>
    <div class="flex flex-col gap-4">
        <!-- Search Filters -->
        <div class="shrink-0">
            <div class="flex items-center justify-between">
                <p class="text-dim text-[11px] font-medium tracking-wider uppercase">Filters</p>
                <button
                    v-if="hasAnyFilter"
                    @click="clearFilters"
                    class="text-dim text-[11px] transition-colors hover:text-white"
                >
                    Clear all
                </button>
            </div>

            <div class="mt-2 grid grid-cols-6 gap-2">
                <!-- Parsing Rules Preset -->
                <div class="col-span-6">
                    <label
                        class="text-dim mb-1 block text-[10px] font-medium tracking-wider uppercase"
                    >
                        Parsing Rules
                    </label>
                    <select
                        v-model="selectedPresetId"
                        @change="handlePresetChange"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full rounded-lg border px-2.5 py-1.5 text-xs text-white transition-all
                            focus:ring-1 focus:outline-none"
                    >
                        <option :value="null">No parsing rules</option>
                        <option
                            v-for="preset in availablePresets"
                            :key="preset.id"
                            :value="preset.id"
                        >
                            {{ preset.name }}
                        </option>
                    </select>
                </div>

                <!-- Title -->
                <div class="col-span-6">
                    <label
                        class="text-dim mb-1 block text-[10px] font-medium tracking-wider uppercase"
                    >
                        Search Query
                    </label>
                    <input
                        v-model="title"
                        type="text"
                        placeholder="Scene title..."
                        class="border-border bg-void/80 placeholder-dim/40 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-2.5 py-1.5 text-xs
                            text-white transition-all focus:ring-1 focus:outline-none"
                        @keydown.enter="searchScenes"
                    />
                </div>

                <!-- Studio -->
                <div class="col-span-2">
                    <label
                        class="text-dim mb-1 block text-[10px] font-medium tracking-wider uppercase"
                    >
                        Studio
                    </label>
                    <input
                        v-model="site"
                        type="text"
                        placeholder="Studio name..."
                        class="border-border bg-void/80 placeholder-dim/40 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-2.5 py-1.5 text-xs
                            text-white transition-all focus:ring-1 focus:outline-none"
                        @keydown.enter="searchScenes"
                    />
                </div>

                <!-- Year -->
                <div class="col-span-1">
                    <label
                        class="text-dim mb-1 block text-[10px] font-medium tracking-wider uppercase"
                    >
                        Year
                    </label>
                    <input
                        v-model="year"
                        type="text"
                        inputmode="numeric"
                        placeholder="2024"
                        class="border-border bg-void/80 placeholder-dim/40 focus:border-lava/40
                            focus:ring-lava/20 w-full rounded-lg border px-2.5 py-1.5 text-xs
                            text-white transition-all focus:ring-1 focus:outline-none"
                        @keydown.enter="searchScenes"
                    />
                </div>

                <!-- Search button -->
                <div class="col-span-3 flex items-end">
                    <button
                        @click="searchScenes"
                        :disabled="searching || !hasAnyFilter"
                        class="bg-lava hover:bg-lava-glow flex w-full items-center justify-center
                            gap-1.5 rounded-lg px-4 py-1.5 text-xs font-semibold text-white
                            transition-all disabled:cursor-not-allowed disabled:opacity-40"
                    >
                        <Icon
                            v-if="searching"
                            name="heroicons:arrow-path"
                            size="14"
                            class="animate-spin"
                        />
                        <template v-else>
                            <Icon name="heroicons:magnifying-glass" size="14" />
                            Search
                        </template>
                    </button>
                </div>
            </div>
        </div>

        <!-- Error -->
        <div
            v-if="searchError"
            class="border-lava/20 bg-lava/5 text-lava shrink-0 rounded-lg border px-3 py-2 text-xs"
        >
            {{ searchError }}
        </div>

        <!-- Results -->
        <div
            v-if="searchResults.length > 0"
            class="border-border min-h-0 flex-1 overflow-y-auto rounded-lg border"
        >
            <div class="sticky top-0 z-10 border-b border-white/5 bg-[#0a0a0a] px-4 py-2">
                <p class="text-dim text-[11px] font-medium tracking-wider uppercase">
                    {{ searchResults.length }} result{{ searchResults.length !== 1 ? 's' : '' }}
                </p>
            </div>
            <div class="divide-y divide-white/5">
                <div
                    v-for="scene in searchResults"
                    :key="scene.id"
                    @click="selectScene(scene)"
                    class="hover:bg-surface flex cursor-pointer gap-4 p-4 transition-colors"
                >
                    <div class="bg-void h-24 w-40 shrink-0 overflow-hidden rounded-lg">
                        <img
                            v-if="scene.image || scene.poster"
                            :src="scene.image || scene.poster"
                            :alt="scene.title"
                            class="h-full w-full object-cover"
                        />
                        <div v-else class="text-dim flex h-full w-full items-center justify-center">
                            <Icon name="heroicons:film" size="24" />
                        </div>
                    </div>
                    <div class="min-w-0 flex-1 py-0.5">
                        <div class="flex items-start justify-between gap-2">
                            <p class="text-sm font-medium text-white">{{ scene.title }}</p>
                            <!-- Confidence Badge -->
                            <div
                                v-if="scene.confidence"
                                :class="getConfidenceColorClass(scene.confidence)"
                                :title="getConfidenceTooltip(scene.confidence)"
                                class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium"
                            >
                                {{ scene.confidence.total }}%
                            </div>
                        </div>
                        <p v-if="scene.site?.name" class="text-dim mt-0.5 text-xs">
                            {{ scene.site.name }}
                        </p>
                        <div
                            class="text-dim mt-2 flex flex-wrap items-center gap-x-3 gap-y-1
                                text-[11px]"
                        >
                            <span v-if="scene.date" class="flex items-center gap-1">
                                <Icon name="heroicons:calendar" size="12" />
                                {{ scene.date }}
                            </span>
                            <span v-if="scene.duration" class="flex items-center gap-1">
                                <Icon name="heroicons:clock" size="12" />
                                {{ formatDuration(scene.duration) }}
                            </span>
                            <span v-if="scene.performers?.length" class="flex items-center gap-1">
                                <Icon name="heroicons:users" size="12" />
                                {{ scene.performers.map((p) => p.name).join(', ') }}
                            </span>
                        </div>
                    </div>
                    <div class="text-dim flex shrink-0 items-center">
                        <Icon name="heroicons:chevron-right" size="16" />
                    </div>
                </div>
            </div>
        </div>

        <!-- Loading indicator -->
        <div v-if="loadingScene" class="flex shrink-0 justify-center py-4">
            <LoadingSpinner />
        </div>

        <!-- Empty state -->
        <div
            v-if="!searching && !loadingScene && searchResults.length === 0"
            class="flex flex-1 items-center justify-center"
        >
            <div class="text-center">
                <Icon name="heroicons:magnifying-glass" size="32" class="text-dim/30 mx-auto" />
                <p class="text-dim mt-2 text-sm">
                    {{
                        hasSearched
                            ? 'No scenes found. Try adjusting your filters.'
                            : 'Fill in filters and search to find scenes.'
                    }}
                </p>
            </div>
        </div>
    </div>
</template>
