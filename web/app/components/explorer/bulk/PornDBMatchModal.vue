<script setup lang="ts">
import type { SceneMatchInfo } from '~/types/explorer';
import type { PornDBScene } from '~/types/porndb';
import type { ParsingPreset } from '~/types/parsing-rules';

const props = defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const settingsStore = useSettingsStore();
const { getScenesMatchInfo } = useApiExplorer();
const { calculateConfidence } = useConfidenceCalculator();
const { getBuiltInPresets } = useParsingRulesEngine();

const {
    results,
    resultsArray,
    isSearching,
    searchProgress,
    applyPhase,
    applyProgress,
    failedScenes,
    matchedCount,
    searchScenes,
    removeMatch,
    applyAllMatched,
    retryFailed,
    clearFailed,
    reset,
} = useBulkPornDBMatching();

const loading = ref(false);
const error = ref<string | null>(null);
const manualSearchScene = ref<SceneMatchInfo | null>(null);
const selectedPresetId = ref<string | null>(null);
const loadedScenes = ref<SceneMatchInfo[]>([]);

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

// Load parsing rules and search when modal opens
onMounted(async () => {
    // Load parsing rules if not already loaded
    if (!settingsStore.parsingRules) {
        await settingsStore.loadParsingRules();
    }
    // Set default preset from settings
    selectedPresetId.value = settingsStore.parsingRules?.activePresetId || null;

    loadAndSearch();
});

async function loadAndSearch() {
    loading.value = true;
    error.value = null;

    try {
        const sceneIds = explorerStore.getSelectedSceneIDs();
        if (sceneIds.length === 0) {
            error.value = 'No scenes selected';
            return;
        }

        // Fetch minimal scene data
        const response = await getScenesMatchInfo(sceneIds);
        const scenes = response.scenes;

        // Store scenes for re-searching when preset changes
        loadedScenes.value = scenes;

        // Filter out already matched scenes from automatic search
        const unmatchedScenes = scenes.filter((s) => !s.porndb_scene_id);

        if (unmatchedScenes.length === 0 && scenes.length > 0) {
            error.value = 'All selected scenes already have PornDB matches';
            return;
        }

        // Start searching with selected preset rules
        await searchScenes(scenes, selectedPresetRules.value);
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to load scenes';
    } finally {
        loading.value = false;
    }
}

// Re-search when preset changes
async function handlePresetChange() {
    // Don't re-search if no scenes loaded, still loading, or already applying
    if (loadedScenes.value.length === 0 || loading.value || applyPhase.value !== 'idle') return;

    // Reset and re-search with new rules
    reset();
    await searchScenes(loadedScenes.value, selectedPresetRules.value);
}

// Watch for preset changes (skip initial value)
watch(selectedPresetId, (_newVal, oldVal) => {
    // Skip if this is the initial setup (oldVal is undefined on first watch trigger)
    if (oldVal === undefined) return;
    handlePresetChange();
});

async function handleApplyAll() {
    await applyAllMatched();
}

async function handleRetryFailed() {
    await retryFailed();
}

function handleClearFailed() {
    clearFailed();
}

function handleClose() {
    if (applyPhase.value === 'done' && applyProgress.value.current > 0) {
        emit('complete');
    }
    emit('close');
}

function openManualSearch(sceneId: number) {
    const result = results.value.get(sceneId);
    if (result) {
        manualSearchScene.value = result.localScene;
    }
}

function onManualSearchSelect(porndbScene: PornDBScene) {
    if (!manualSearchScene.value) return;

    const sceneId = manualSearchScene.value.id;
    const existing = results.value.get(sceneId);
    if (existing) {
        // Calculate confidence for manual selection
        const confidence = calculateConfidence(existing.localScene, porndbScene);

        // Update result with new match
        results.value.set(sceneId, {
            ...existing,
            match: porndbScene,
            confidence,
            status: 'matched',
        });
    }
    manualSearchScene.value = null;
}

// Summary stats
const skippedCount = computed(() => {
    return resultsArray.value.filter((r) => r.status === 'skipped').length;
});

const noMatchCount = computed(() => {
    return resultsArray.value.filter((r) => r.status === 'no-match' || r.status === 'removed')
        .length;
});

const appliedCount = computed(() => {
    return resultsArray.value.filter((r) => r.status === 'applied').length;
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div
                class="border-border bg-panel flex h-[85vh] w-full max-w-6xl flex-col rounded-xl
                    border shadow-2xl"
            >
                <!-- Header -->
                <div
                    class="border-border flex shrink-0 items-center justify-between border-b px-4
                        py-3"
                >
                    <div class="flex items-center gap-3">
                        <h2 class="text-sm font-semibold text-white">Bulk Match with ThePornDB</h2>
                        <!-- Preset selector -->
                        <select
                            v-model="selectedPresetId"
                            :disabled="isSearching || loading"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                rounded border px-2 py-1 text-xs text-white transition-all
                                focus:ring-1 focus:outline-none disabled:opacity-50"
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
                        <!-- Status badge -->
                        <div v-if="isSearching" class="flex items-center gap-2">
                            <LoadingSpinner class="h-4 w-4" />
                            <span class="text-dim text-xs">
                                Searching {{ searchProgress.current }}/{{ searchProgress.total }}...
                            </span>
                        </div>
                        <div v-else-if="!loading" class="text-dim text-xs">
                            {{ matchedCount }} matches found
                            <span v-if="skippedCount > 0">
                                &middot; {{ skippedCount }} skipped</span
                            >
                            <span v-if="noMatchCount > 0">
                                &middot; {{ noMatchCount }} no match</span
                            >
                            <span v-if="appliedCount > 0" class="text-emerald-400">
                                &middot; {{ appliedCount }} applied
                            </span>
                        </div>
                    </div>
                    <button
                        @click="handleClose"
                        class="text-dim transition-colors hover:text-white"
                    >
                        <Icon name="heroicons:x-mark" size="20" />
                    </button>
                </div>

                <!-- Content -->
                <div class="flex-1 overflow-hidden">
                    <!-- Loading state -->
                    <div v-if="loading" class="flex h-full items-center justify-center">
                        <div class="text-center">
                            <LoadingSpinner class="mx-auto" />
                            <p class="text-dim mt-3 text-sm">Loading scene data...</p>
                        </div>
                    </div>

                    <!-- Error state -->
                    <div v-else-if="error" class="flex h-full items-center justify-center p-8">
                        <div class="text-center">
                            <Icon
                                name="heroicons:exclamation-circle"
                                size="48"
                                class="mx-auto text-red-400"
                            />
                            <p class="mt-3 text-sm text-red-400">{{ error }}</p>
                            <button
                                @click="handleClose"
                                class="border-border hover:border-border-hover mt-4 rounded-lg
                                    border px-4 py-2 text-xs font-medium text-white transition-all"
                            >
                                Close
                            </button>
                        </div>
                    </div>

                    <!-- Results list -->
                    <div v-else class="h-full overflow-y-auto p-4">
                        <div class="space-y-2">
                            <ExplorerBulkPornDBMatchRow
                                v-for="result in resultsArray"
                                :key="result.sceneId"
                                :result="result"
                                @manual-search="openManualSearch"
                                @remove-match="removeMatch"
                            />
                        </div>
                    </div>
                </div>

                <!-- Apply Progress -->
                <ExplorerBulkPornDBMatchProgress
                    v-if="applyPhase !== 'idle'"
                    :phase="applyPhase"
                    :progress="applyProgress"
                />

                <!-- Retry Queue -->
                <ExplorerBulkPornDBMatchRetryQueue
                    v-if="failedScenes.length > 0 && applyPhase === 'done'"
                    :failed-scenes="failedScenes"
                    @retry="handleRetryFailed"
                    @dismiss="handleClearFailed"
                />

                <!-- Footer -->
                <div
                    class="border-border flex shrink-0 items-center justify-end gap-3 border-t px-4
                        py-3"
                >
                    <button
                        @click="handleClose"
                        class="border-border hover:border-border-hover rounded-lg border px-4 py-2
                            text-xs font-medium text-white transition-all"
                    >
                        {{ applyPhase === 'done' ? 'Done' : 'Cancel' }}
                    </button>
                    <button
                        v-if="applyPhase === 'idle'"
                        @click="handleApplyAll"
                        :disabled="matchedCount === 0 || isSearching || loading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-colors disabled:cursor-not-allowed
                            disabled:opacity-50"
                    >
                        Apply {{ matchedCount }} Matches
                    </button>
                </div>
            </div>

            <!-- Manual search modal -->
            <ExplorerBulkPornDBSearchModal
                v-if="manualSearchScene"
                :visible="!!manualSearchScene"
                :scene="manualSearchScene"
                @close="manualSearchScene = null"
                @select="onManualSearchSelect"
            />
        </div>
    </Teleport>
</template>
