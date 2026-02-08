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
const { applyRules, getAllPresets } = useParsingRulesEngine();

const title = ref('');
const selectedPresetId = ref<string | null>(null);
const showParsingOptions = ref(false);

const searching = ref(false);
const hasSearched = ref(false);
const searchResults = ref<SearchResultWithConfidence[]>([]);
const searchError = ref('');
const loadingSceneId = ref<string | null>(null);

// Available presets (stored user presets + hardcoded fallbacks)
const availablePresets = computed(() => {
    return getAllPresets(settingsStore.parsingRules);
});

// Get rules for selected preset
const selectedPresetRules = computed(() => {
    if (!selectedPresetId.value) return undefined;
    const preset = availablePresets.value.find((p) => p.id === selectedPresetId.value);
    return preset?.rules;
});

// Check if scene has enough data for confidence calculation
const canCalculateConfidence = computed(() => {
    return (
        props.scene &&
        (props.scene.original_filename || props.scene.duration || props.scene.actors?.length)
    );
});

const hasAnyFilter = computed(() => {
    return title.value.trim() !== '';
});

function formatDuration(seconds: number): string {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function clearFilters() {
    title.value = '';
}

function getQueryFromScene(): string {
    if (!props.scene) return '';

    // If parsing rules are selected, apply them to original_filename
    let query = '';
    if (selectedPresetRules.value && props.scene.original_filename) {
        query = applyRules(props.scene.original_filename, selectedPresetRules.value);
    } else {
        query = props.scene.title || '';
    }

    const queryLower = query.toLowerCase();

    // Append actor names if not already in query
    if (props.scene.actors?.length) {
        for (const actor of props.scene.actors) {
            if (!queryLower.includes(actor.toLowerCase())) {
                query += ` ${actor}`;
            }
        }
    }

    // Append studio name if not already in query
    if (props.scene.studio && !queryLower.includes(props.scene.studio.toLowerCase())) {
        query += ` ${props.scene.studio}`;
    }

    return query.trim();
}

function populateFromScene() {
    if (!props.scene) return;

    title.value = getQueryFromScene();
}

async function searchScenes() {
    if (!hasAnyFilter.value) return;

    searching.value = true;
    searchError.value = '';
    searchResults.value = [];
    hasSearched.value = true;

    try {
        const results = await api.searchPornDBScenes({ title: title.value.trim() });

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
    if (confidence.total >= 80) return 'bg-emerald-500/20 text-emerald-400';
    if (confidence.total >= 50) return 'bg-amber-500/20 text-amber-400';
    return 'bg-red-500/20 text-red-400';
}

function getConfidenceTooltip(confidence: ConfidenceBreakdown | undefined): string {
    if (!confidence) return '';
    return `Title: ${confidence.titleScore}/30, Actors: ${confidence.actorScore}/30, Studio: ${confidence.studioScore}/20, Duration: ${confidence.durationScore}/20`;
}

function getSegmentColor(score: number, max: number): string {
    const ratio = score / max;
    if (ratio >= 0.7) return 'bg-emerald-500/70';
    if (ratio >= 0.4) return 'bg-amber-500/50';
    if (ratio > 0) return 'bg-red-500/40';
    return 'bg-white/[0.06]';
}

// Re-populate search field when preset changes
function handlePresetChange() {
    title.value = getQueryFromScene();
}

async function selectScene(scene: PornDBScene) {
    loadingSceneId.value = scene.id;

    try {
        const details = await api.getPornDBScene(scene.id);
        emit('select', details);
    } catch (e: unknown) {
        searchError.value = e instanceof Error ? e.message : 'Failed to load scene details';
    } finally {
        loadingSceneId.value = null;
    }
}

onMounted(async () => {
    // Load parsing rules if not already loaded
    if (!settingsStore.parsingRules) {
        await settingsStore.loadParsingRules();
    }
    // Set default preset from settings
    selectedPresetId.value = settingsStore.parsingRules?.activePresetId || null;

    // Auto-show parsing options if a preset is active
    if (selectedPresetId.value) {
        showParsingOptions.value = true;
    }

    populateFromScene();

    // Auto-search if query was populated from scene data
    if (hasAnyFilter.value) {
        await searchScenes();
    }
});
</script>

<template>
    <div class="flex h-full max-h-[75vh] flex-col gap-3">
        <!-- Search Area -->
        <div class="shrink-0">
            <!-- Search bar row -->
            <div class="flex items-center gap-2">
                <div
                    class="border-border bg-void/80 flex min-w-0 flex-1 items-center rounded-lg
                        border transition-colors focus-within:border-white/15"
                >
                    <Icon
                        name="heroicons:magnifying-glass"
                        size="14"
                        class="text-dim ml-3 shrink-0"
                    />
                    <input
                        v-model="title"
                        type="text"
                        placeholder="Search PornDB..."
                        class="min-w-0 flex-1 bg-transparent px-2.5 py-2 text-xs text-white
                            placeholder-white/20 focus:outline-none"
                        @keydown.enter="searchScenes"
                    />
                    <button
                        v-if="title"
                        class="text-dim flex shrink-0 items-center pr-2.5 transition-colors
                            hover:text-white"
                        @click="clearFilters"
                    >
                        <Icon name="heroicons:x-circle-solid" size="14" />
                    </button>
                </div>

                <button
                    :disabled="searching || !hasAnyFilter"
                    class="bg-lava hover:bg-lava-glow flex h-8 shrink-0 items-center gap-1.5
                        rounded-lg px-4 text-xs font-medium text-white transition-all
                        disabled:cursor-not-allowed disabled:opacity-30"
                    @click="searchScenes"
                >
                    <Icon
                        v-if="searching"
                        name="heroicons:arrow-path"
                        size="13"
                        class="animate-spin"
                    />
                    <template v-else>Search</template>
                </button>
            </div>

            <!-- Parsing rules toggle -->
            <div class="mt-2 flex items-center gap-2">
                <button
                    class="text-dim flex items-center gap-1 text-[10px] tracking-wider uppercase
                        transition-colors hover:text-white/60"
                    @click="showParsingOptions = !showParsingOptions"
                >
                    <Icon
                        name="heroicons:chevron-right"
                        size="10"
                        class="transition-transform duration-150"
                        :class="{ 'rotate-90': showParsingOptions }"
                    />
                    Parsing Rules
                </button>
                <span
                    v-if="selectedPresetId"
                    class="bg-lava/10 text-lava rounded px-1.5 py-px text-[10px] leading-normal"
                >
                    Active
                </span>
            </div>

            <!-- Parsing rules select (collapsible) -->
            <div v-if="showParsingOptions" class="mt-1.5">
                <select
                    v-model="selectedPresetId"
                    class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                        rounded-lg border px-2.5 py-1.5 text-xs text-white transition-all
                        focus:ring-1 focus:outline-none"
                    @change="handlePresetChange"
                >
                    <option :value="null">No parsing rules</option>
                    <option v-for="preset in availablePresets" :key="preset.id" :value="preset.id">
                        {{ preset.name }}
                    </option>
                </select>
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
            <!-- Sticky result count -->
            <div
                class="bg-surface/95 sticky top-0 z-10 border-b border-white/5 px-3 py-1.5
                    backdrop-blur-sm"
            >
                <p class="text-dim text-[11px] font-medium tracking-wider uppercase">
                    {{ searchResults.length }} result{{ searchResults.length !== 1 ? 's' : '' }}
                </p>
            </div>

            <!-- Result cards -->
            <div class="divide-y divide-white/4">
                <div
                    v-for="result in searchResults"
                    :key="result.id"
                    class="group relative flex cursor-pointer gap-3 p-3 transition-all
                        hover:bg-white/3"
                    @click="selectScene(result)"
                >
                    <!-- Loading overlay for selected scene -->
                    <div
                        v-if="loadingSceneId === result.id"
                        class="absolute inset-0 z-10 flex items-center justify-center bg-black/50
                            backdrop-blur-sm"
                    >
                        <LoadingSpinner />
                    </div>

                    <!-- Thumbnail -->
                    <div class="relative h-20 w-36 shrink-0 overflow-hidden rounded-md bg-white/3">
                        <img
                            v-if="result.image || result.poster"
                            :src="result.image || result.poster"
                            :alt="result.title"
                            class="h-full w-full object-cover transition-transform duration-300
                                group-hover:scale-[1.03]"
                        />
                        <div
                            v-else
                            class="flex h-full w-full items-center justify-center text-white/10"
                        >
                            <Icon name="heroicons:film" size="20" />
                        </div>
                    </div>

                    <!-- Content -->
                    <div class="min-w-0 flex-1 py-0.5">
                        <p
                            class="truncate text-[13px] font-medium text-white
                                group-hover:text-white"
                        >
                            {{ result.title }}
                        </p>

                        <!-- Confidence score + breakdown bar -->
                        <div
                            v-if="result.confidence"
                            :title="getConfidenceTooltip(result.confidence)"
                            class="mt-1.5 flex items-center gap-2"
                        >
                            <div
                                class="flex h-1 min-w-0 flex-1 gap-px overflow-hidden rounded-full"
                            >
                                <div
                                    class="flex-30 rounded-full transition-colors"
                                    :class="getSegmentColor(result.confidence.titleScore, 30)"
                                />
                                <div
                                    class="flex-30 rounded-full transition-colors"
                                    :class="getSegmentColor(result.confidence.actorScore, 30)"
                                />
                                <div
                                    class="flex-20 rounded-full transition-colors"
                                    :class="getSegmentColor(result.confidence.studioScore, 20)"
                                />
                                <div
                                    class="flex-20 rounded-full transition-colors"
                                    :class="getSegmentColor(result.confidence.durationScore, 20)"
                                />
                            </div>
                            <span
                                :class="getConfidenceColorClass(result.confidence)"
                                class="shrink-0 rounded px-1.5 py-0.5 text-[10px] leading-none
                                    font-semibold"
                            >
                                {{ result.confidence.total }}%
                            </span>
                        </div>

                        <!-- Metadata -->
                        <div
                            class="text-dim mt-2 flex flex-wrap items-center gap-x-3 gap-y-1
                                text-[11px]"
                        >
                            <span v-if="result.site?.name" class="text-white/50">
                                {{ result.site.name }}
                            </span>
                            <span v-if="result.date" class="flex items-center gap-1">
                                <Icon name="heroicons:calendar" size="11" />
                                {{ result.date }}
                            </span>
                            <span v-if="result.duration" class="flex items-center gap-1">
                                <Icon name="heroicons:clock" size="11" />
                                {{ formatDuration(result.duration) }}
                            </span>
                            <span v-if="result.performers?.length" class="flex items-center gap-1">
                                <Icon name="heroicons:users" size="11" />
                                {{ result.performers.map((p) => p.name).join(', ') }}
                            </span>
                        </div>
                    </div>

                    <!-- Arrow (visible on hover) -->
                    <div
                        class="text-dim flex shrink-0 items-center opacity-0 transition-opacity
                            group-hover:opacity-100"
                    >
                        <Icon name="heroicons:chevron-right" size="14" />
                    </div>
                </div>
            </div>
        </div>

        <!-- Searching skeleton -->
        <div v-else-if="searching" class="min-h-0 flex-1 space-y-1 overflow-hidden pt-1">
            <div
                v-for="i in 5"
                :key="i"
                class="flex gap-3 p-3"
                :style="{ opacity: 1 - (i - 1) * 0.15 }"
            >
                <div class="h-20 w-36 shrink-0 animate-pulse rounded-md bg-white/4" />
                <div class="flex-1 space-y-2.5 py-1">
                    <div class="h-3.5 w-3/4 animate-pulse rounded bg-white/6" />
                    <div class="h-1 w-24 animate-pulse rounded-full bg-white/4" />
                    <div class="flex gap-3 pt-1">
                        <div class="h-2.5 w-16 animate-pulse rounded bg-white/4" />
                        <div class="h-2.5 w-12 animate-pulse rounded bg-white/4" />
                        <div class="h-2.5 w-20 animate-pulse rounded bg-white/4" />
                    </div>
                </div>
            </div>
        </div>

        <!-- Empty state -->
        <div v-else class="flex flex-1 items-center justify-center">
            <div class="text-center">
                <div
                    class="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full
                        border border-white/6 text-white/10"
                >
                    <Icon name="heroicons:magnifying-glass" size="18" />
                </div>
                <p class="text-dim text-xs">
                    {{
                        hasSearched
                            ? 'No results found. Try a different query.'
                            : 'Search PornDB for scene metadata.'
                    }}
                </p>
            </div>
        </div>
    </div>
</template>
