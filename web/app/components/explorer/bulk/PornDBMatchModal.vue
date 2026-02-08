<script setup lang="ts">
import type { SceneMatchInfo } from '~/types/explorer';
import type { PornDBScene } from '~/types/porndb';
const props = defineProps<{
    visible: boolean;
    sceneIds?: number[];
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const settingsStore = useSettingsStore();
const { getScenesMatchInfo } = useApiExplorer();
const { calculateConfidence } = useConfidenceCalculator();
const { getAllPresets } = useParsingRulesEngine();

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
        const sceneIds = props.sceneIds ?? explorerStore.getSelectedSceneIDs();
        if (sceneIds.length === 0) {
            error.value = 'No scenes selected';
            loading.value = false;
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
            loading.value = false;
            return;
        }

        // Done loading scene data - show results immediately as search progresses
        loading.value = false;

        // Start searching with selected preset rules (don't await - let results stream in)
        searchScenes(scenes, selectedPresetRules.value);
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to load scenes';
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

const showProgressOverlay = ref(false);

async function handleApplyAll() {
    showProgressOverlay.value = true;
    await applyAllMatched();
}

function handleProgressContinue() {
    showProgressOverlay.value = false;
    // If no failures, close the modal entirely
    if (failedScenes.value.length === 0) {
        handleClose();
    }
}

async function handleRetryFailed() {
    showProgressOverlay.value = true;
    await retryFailed();
}

function handleClearFailed() {
    clearFailed();
}

// Whether there's active work that would be lost on close
const hasUnsavedWork = computed(() => {
    if (isSearching.value) return true;
    if (applyPhase.value === 'applying') return true;
    if (matchedCount.value > 0 && applyPhase.value === 'idle') return true;
    return false;
});

function handleClose() {
    if (hasUnsavedWork.value) {
        if (!window.confirm('You have unapplied matches. Are you sure you want to close?')) return;
    }
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

const hideSkipped = ref(true);

const filteredResults = computed(() => {
    if (!hideSkipped.value) return resultsArray.value;
    return resultsArray.value.filter((r) => r.status !== 'skipped');
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="!hasUnsavedWork && handleClose()"
        >
            <div
                class="border-border bg-panel relative flex h-[85vh] w-full max-w-6xl flex-col
                    overflow-hidden rounded-xl border shadow-2xl"
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
                        <!-- Status badges -->
                        <div v-if="isSearching" class="flex items-center gap-1.5">
                            <LoadingSpinner class="scale-50" />
                            <span class="text-dim text-xs">
                                Searching {{ searchProgress.current }}/{{ searchProgress.total }}...
                            </span>
                        </div>
                        <div v-else-if="!loading" class="flex items-center gap-1.5">
                            <span
                                v-if="matchedCount > 0"
                                class="inline-flex items-center gap-1 rounded-full bg-emerald-500/15
                                    px-2 py-0.5 text-xs font-medium text-emerald-400"
                            >
                                <Icon name="heroicons:check-circle-16-solid" size="12" />
                                {{ matchedCount }} matched
                            </span>
                            <span
                                v-if="skippedCount > 0"
                                class="inline-flex items-center gap-1 rounded-full bg-amber-500/15
                                    px-2 py-0.5 text-xs font-medium text-amber-400"
                            >
                                <Icon name="heroicons:arrow-right-circle-16-solid" size="12" />
                                {{ skippedCount }} skipped
                            </span>
                            <span
                                v-if="noMatchCount > 0"
                                class="text-dim inline-flex items-center gap-1 rounded-full
                                    bg-white/5 px-2 py-0.5 text-xs font-medium"
                            >
                                <Icon name="heroicons:x-circle-16-solid" size="12" />
                                {{ noMatchCount }} no match
                            </span>
                            <span
                                v-if="appliedCount > 0"
                                class="inline-flex items-center gap-1 rounded-full bg-sky-500/15
                                    px-2 py-0.5 text-xs font-medium text-sky-400"
                            >
                                <Icon name="heroicons:cloud-arrow-up-16-solid" size="12" />
                                {{ appliedCount }} applied
                            </span>
                        </div>
                    </div>
                    <button
                        class="text-dim flex items-center transition-colors hover:text-white"
                        @click="handleClose"
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
                                class="border-border hover:border-border-hover mt-4 rounded-lg
                                    border px-4 py-2 text-xs font-medium text-white transition-all"
                                @click="handleClose"
                            >
                                Close
                            </button>
                        </div>
                    </div>

                    <!-- Results list -->
                    <div v-else class="flex h-full flex-col">
                        <!-- Filter bar -->
                        <div
                            v-if="skippedCount > 0"
                            class="border-border shrink-0 border-b px-4 py-2"
                        >
                            <label class="text-dim flex items-center gap-2 text-xs">
                                <input
                                    v-model="hideSkipped"
                                    type="checkbox"
                                    class="accent-lava h-3 w-3 rounded"
                                />
                                Hide already matched ({{ skippedCount }})
                            </label>
                        </div>
                        <div class="min-h-0 flex-1 overflow-y-auto p-4">
                            <div class="space-y-2">
                                <ExplorerBulkPornDBMatchRow
                                    v-for="result in filteredResults"
                                    :key="result.sceneId"
                                    :result="result"
                                    @manual-search="openManualSearch"
                                    @remove-match="removeMatch"
                                />
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Footer -->
                <div
                    v-if="!showProgressOverlay"
                    class="border-border flex shrink-0 items-center justify-end gap-3 border-t px-4
                        py-3"
                >
                    <button
                        class="border-border hover:border-border-hover rounded-lg border px-4 py-2
                            text-xs font-medium text-white transition-all"
                        @click="handleClose"
                    >
                        {{ applyPhase === 'done' ? 'Done' : 'Cancel' }}
                    </button>
                    <button
                        v-if="applyPhase === 'idle'"
                        :disabled="matchedCount === 0 || isSearching || loading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-colors disabled:cursor-not-allowed
                            disabled:opacity-50"
                        @click="handleApplyAll"
                    >
                        Apply {{ matchedCount }} Matches
                    </button>
                </div>

                <!-- Apply Progress Overlay -->
                <ExplorerBulkPornDBMatchProgress
                    :phase="showProgressOverlay ? applyPhase : 'idle'"
                    :progress="applyProgress"
                    @close="handleProgressContinue"
                />

                <!-- Retry Queue (shown after overlay dismisses when there are failures) -->
                <ExplorerBulkPornDBMatchRetryQueue
                    v-if="failedScenes.length > 0 && applyPhase === 'done' && !showProgressOverlay"
                    :failed-scenes="failedScenes"
                    @retry="handleRetryFailed"
                    @dismiss="handleClearFailed"
                />
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
