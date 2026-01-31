<script setup lang="ts">
import type { Marker, MarkerLabelSuggestion } from '~/types/marker';

const route = useRoute();
const { fetchMarkers, createMarker, updateMarker, deleteMarker, fetchLabelSuggestions } =
    useApiMarkers();
const { formatDuration } = useFormatter();
const getPlayerTime = inject<() => number>('getPlayerTime');
const seekToTime = inject<(time: number) => void>('seekToTime');
const refreshMarkers = inject<() => Promise<void>>('refreshMarkers');

const videoId = computed(() => parseInt(route.params.id as string));

const loading = ref(true);
const markers = ref<Marker[]>([]);
const newMarkerLabel = ref('');
const saving = ref(false);
const editInputRef = ref<HTMLInputElement[]>([]);
const newMarkerInputRef = ref<HTMLInputElement | null>(null);
const editingMarkerId = ref<number | null>(null);
const editingLabel = ref('');
const error = ref<string | null>(null);

// Delete confirmation state
const markerToDelete = ref<Marker | null>(null);
const showDeleteConfirm = ref(false);

// Autocomplete state
const labelSuggestions = ref<MarkerLabelSuggestion[]>([]);
const suggestionsLoaded = ref(false);
const showNewMarkerSuggestions = ref(false);
const showEditSuggestions = ref(false);
const selectedSuggestionIndex = ref(-1);

const loadMarkers = async () => {
    loading.value = true;
    error.value = null;
    try {
        const data = await fetchMarkers(videoId.value);
        markers.value = data.markers || [];
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to load markers';
    } finally {
        loading.value = false;
    }
};

const loadSuggestions = async () => {
    if (suggestionsLoaded.value) return;
    try {
        const data = await fetchLabelSuggestions();
        labelSuggestions.value = data.labels || [];
        suggestionsLoaded.value = true;
    } catch (e) {
        // Silently fail - autocomplete is a nice-to-have
    }
};

const filteredSuggestions = computed(() => {
    const query = showEditSuggestions.value ? editingLabel.value : newMarkerLabel.value;
    if (!query.trim()) return labelSuggestions.value;
    const lowerQuery = query.toLowerCase();
    return labelSuggestions.value.filter((s) => s.label.toLowerCase().includes(lowerQuery));
});

const handleNewMarkerFocus = () => {
    loadSuggestions();
    showNewMarkerSuggestions.value = true;
    selectedSuggestionIndex.value = -1;
};

const handleNewMarkerBlur = () => {
    // Delay hiding to allow click on suggestion
    setTimeout(() => {
        showNewMarkerSuggestions.value = false;
        selectedSuggestionIndex.value = -1;
    }, 150);
};

const handleEditFocus = () => {
    loadSuggestions();
    showEditSuggestions.value = true;
    selectedSuggestionIndex.value = -1;
};

const handleEditBlur = (marker: Marker) => {
    setTimeout(() => {
        showEditSuggestions.value = false;
        selectedSuggestionIndex.value = -1;
        // Auto-save on blur if still editing this marker
        if (editingMarkerId.value === marker.id) {
            handleSaveEdit(marker);
        }
    }, 150);
};

const selectSuggestion = (suggestion: MarkerLabelSuggestion) => {
    if (showEditSuggestions.value) {
        editingLabel.value = suggestion.label;
        showEditSuggestions.value = false;
    } else {
        newMarkerLabel.value = suggestion.label;
        showNewMarkerSuggestions.value = false;
    }
    selectedSuggestionIndex.value = -1;
};

const handleSuggestionKeydown = (e: KeyboardEvent, isEdit: boolean) => {
    const suggestions = filteredSuggestions.value;
    if (!suggestions.length) return;

    if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedSuggestionIndex.value = Math.min(
            selectedSuggestionIndex.value + 1,
            suggestions.length - 1,
        );
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedSuggestionIndex.value = Math.max(selectedSuggestionIndex.value - 1, -1);
    } else if (e.key === 'Enter' && selectedSuggestionIndex.value >= 0) {
        e.preventDefault();
        const suggestion = suggestions[selectedSuggestionIndex.value];
        if (suggestion) {
            selectSuggestion(suggestion);
        }
    } else if (e.key === 'Escape') {
        if (isEdit) {
            showEditSuggestions.value = false;
        } else {
            showNewMarkerSuggestions.value = false;
        }
        selectedSuggestionIndex.value = -1;
    }
};

const handleAddMarker = async () => {
    if (saving.value) return;

    const currentTime = getPlayerTime?.() ?? 0;
    saving.value = true;
    error.value = null;

    try {
        const marker = await createMarker(videoId.value, {
            timestamp: Math.floor(currentTime),
            label: newMarkerLabel.value.trim() || undefined,
        });
        markers.value = [...markers.value, marker].sort((a, b) => a.timestamp - b.timestamp);
        newMarkerLabel.value = '';
        // Start editing the new marker's label
        editingMarkerId.value = marker.id;
        editingLabel.value = marker.label;
        // Wait for DOM update and focus the edit input (with fallback if ref not ready)
        nextTick(() => {
            const input = editInputRef.value?.[0];
            if (input) {
                input.focus();
            }
        });
        // Refresh parent markers for timeline indicators
        refreshMarkers?.();
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to add marker';
    } finally {
        saving.value = false;
    }
};

const handleSeekToMarker = (timestamp: number) => {
    if (seekToTime) {
        seekToTime(timestamp);
        window.scrollTo({ top: 0, behavior: 'smooth' });
        const playerEl = document.getElementById('video-player');
        if (playerEl) {
            playerEl.focus();
        }
    }
};

const startEditing = (marker: Marker, event: Event) => {
    event.stopPropagation();
    editingMarkerId.value = marker.id;
    editingLabel.value = marker.label;
    nextTick(() => {
        const input = editInputRef.value?.[0];
        if (input) {
            input.focus();
        }
    });
};

const cancelEditing = () => {
    editingMarkerId.value = null;
    editingLabel.value = '';
};

const handleSaveEdit = async (marker: Marker) => {
    if (editingLabel.value === marker.label) {
        cancelEditing();
        return;
    }

    error.value = null;
    try {
        const updated = await updateMarker(videoId.value, marker.id, {
            label: editingLabel.value.trim(),
        });
        const index = markers.value.findIndex((m) => m.id === marker.id);
        if (index !== -1) {
            markers.value[index] = updated;
        }
        cancelEditing();
        // Refresh parent markers for timeline indicators
        refreshMarkers?.();
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to update marker';
    }
};

const handleDeleteClick = (marker: Marker, event: Event) => {
    event.stopPropagation();
    markerToDelete.value = marker;
    showDeleteConfirm.value = true;
};

const cancelDelete = () => {
    markerToDelete.value = null;
    showDeleteConfirm.value = false;
};

const confirmDelete = async () => {
    if (!markerToDelete.value) return;

    const markerId = markerToDelete.value.id;
    showDeleteConfirm.value = false;
    error.value = null;

    try {
        await deleteMarker(videoId.value, markerId);
        markers.value = markers.value.filter((m) => m.id !== markerId);
        // Refresh parent markers for timeline indicators
        refreshMarkers?.();
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to delete marker';
    } finally {
        markerToDelete.value = null;
    }
};

const getThumbnailUrl = (marker: Marker) => {
    if (!marker.thumbnail_path) return null;
    return `/marker-thumbnails/${marker.id}`;
};

const handleKeydown = (e: KeyboardEvent) => {
    // Ignore if typing in an input/textarea or if modifier keys are pressed
    const target = e.target as HTMLElement;
    if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable ||
        e.ctrlKey ||
        e.metaKey ||
        e.altKey
    ) {
        return;
    }

    // 'M' key to add marker at current time
    if (e.key === 'm' || e.key === 'M') {
        e.preventDefault();
        handleAddMarker();
    }
};

onMounted(() => {
    loadMarkers();
    window.addEventListener('keydown', handleKeydown);
});

onUnmounted(() => {
    window.removeEventListener('keydown', handleKeydown);
});
</script>

<template>
    <div class="space-y-4">
        <!-- Add Marker Form -->
        <div class="flex items-center gap-2">
            <div class="flex-1">
                <input
                    ref="newMarkerInputRef"
                    v-model="newMarkerLabel"
                    type="text"
                    placeholder="Label (optional)"
                    maxlength="100"
                    class="border-border bg-surface text-dim placeholder:text-dim/50 w-full
                        rounded-lg border px-3 py-2 text-xs transition-colors focus:border-white/20
                        focus:text-white focus:outline-none"
                    @focus="handleNewMarkerFocus"
                    @blur="handleNewMarkerBlur"
                    @keydown="handleSuggestionKeydown($event, false)"
                    @keyup.enter="selectedSuggestionIndex < 0 && handleAddMarker()"
                />
            </div>
            <button
                :disabled="saving"
                class="bg-lava hover:bg-lava/80 disabled:bg-lava/50 flex items-center gap-1.5
                    rounded-lg px-3 py-2 text-xs font-medium text-white transition-colors
                    disabled:cursor-not-allowed"
                title="Add marker at current time (M)"
                @click="handleAddMarker"
            >
                <Icon v-if="saving" name="svg-spinners:ring-resize" size="12" />
                <Icon v-else name="heroicons:plus" size="12" />
                Add Marker
                <kbd
                    class="ml-1 rounded bg-white/10 px-1.5 py-0.5 font-mono text-[10px] font-normal"
                    >M</kbd
                >
            </button>
        </div>

        <!-- Error Alert -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-white">{{ error }}</span>
            <button class="text-dim ml-auto text-xs hover:text-white" @click="error = null">
                Dismiss
            </button>
        </div>

        <!-- Loading state -->
        <div v-if="loading" class="text-dim py-4 text-center text-[11px]">Loading markers...</div>

        <!-- Empty state -->
        <div v-else-if="markers.length === 0" class="text-dim py-4 text-center text-[11px]">
            No markers yet. Add one to bookmark a moment.
        </div>

        <!-- Markers Grid -->
        <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            <div
                v-for="marker in markers"
                :key="marker.id"
                class="border-border bg-surface group relative overflow-hidden rounded-lg border
                    transition-all hover:border-white/20"
            >
                <!-- Thumbnail / Placeholder -->
                <button
                    class="relative aspect-video w-full cursor-pointer overflow-hidden bg-black/40"
                    :title="`Seek to ${formatDuration(marker.timestamp)}`"
                    @click="handleSeekToMarker(marker.timestamp)"
                >
                    <!-- Thumbnail Image -->
                    <img
                        v-if="getThumbnailUrl(marker)"
                        :src="getThumbnailUrl(marker)!"
                        :alt="marker.label || 'Marker thumbnail'"
                        class="h-full w-full object-cover transition-transform duration-200
                            group-hover:scale-105"
                    />
                    <!-- Placeholder when no thumbnail -->
                    <div
                        v-else
                        class="flex h-full w-full items-center justify-center bg-linear-to-br
                            from-white/5 to-transparent"
                    >
                        <Icon name="heroicons:photo" size="24" class="text-dim/40" />
                    </div>

                    <!-- Timestamp Badge Overlay (bottom-right) -->
                    <div
                        class="absolute right-2 bottom-2 rounded bg-black/70 px-1.5 py-0.5 font-mono
                            text-[10px] text-white backdrop-blur-sm"
                    >
                        {{ formatDuration(marker.timestamp) }}
                    </div>

                    <!-- Play Icon on Hover -->
                    <div
                        class="bg-lava/25 absolute inset-0 flex items-center justify-center
                            opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                    >
                        <Icon name="heroicons:play-solid" size="28" class="text-white" />
                    </div>
                </button>

                <!-- Label & Actions -->
                <div class="flex items-center gap-2 p-2 py-0.5">
                    <!-- Label (editable) -->
                    <div class="min-w-0 flex-1">
                        <template v-if="editingMarkerId === marker.id">
                            <div class="flex items-center gap-1">
                                <div class="min-w-0 flex-1">
                                    <input
                                        ref="editInputRef"
                                        v-model="editingLabel"
                                        type="text"
                                        maxlength="100"
                                        class="border-border bg-panel w-full rounded-md border px-2
                                            py-1 text-[11px] text-white focus:border-white/20
                                            focus:outline-none"
                                        @focus="handleEditFocus"
                                        @blur="handleEditBlur(marker)"
                                        @keydown="handleSuggestionKeydown($event, true)"
                                        @keyup.enter="
                                            selectedSuggestionIndex < 0 && handleSaveEdit(marker)
                                        "
                                        @keyup.escape="cancelEditing"
                                    />
                                </div>
                                <button
                                    class="text-emerald hover:bg-emerald/10 shrink-0 rounded p-1
                                        transition-colors"
                                    title="Save"
                                    @click="handleSaveEdit(marker)"
                                >
                                    <Icon name="heroicons:check" size="12" />
                                </button>
                                <button
                                    class="text-dim hover:bg-surface-hover shrink-0 rounded p-1
                                        transition-colors"
                                    title="Cancel"
                                    @click="cancelEditing"
                                >
                                    <Icon name="heroicons:x-mark" size="12" />
                                </button>
                            </div>
                        </template>
                        <template v-else>
                            <span
                                v-if="marker.label"
                                class="text-dim line-clamp-1 cursor-pointer text-[11px]
                                    transition-colors hover:text-white"
                                :title="marker.label"
                                @click="startEditing(marker, $event)"
                            >
                                {{ marker.label }}
                            </span>
                            <span
                                v-else
                                class="text-dim/50 cursor-pointer text-[11px] italic
                                    transition-colors hover:text-white/50"
                                @click="startEditing(marker, $event)"
                            >
                                No label
                            </span>
                        </template>
                    </div>

                    <!-- Delete Button -->
                    <button
                        v-if="editingMarkerId !== marker.id"
                        class="text-dim hover:text-lava hover:bg-lava/10 flex shrink-0 items-center
                            justify-center rounded p-1.5 opacity-0 transition-all
                            group-hover:opacity-100"
                        title="Delete marker"
                        @click="handleDeleteClick(marker, $event)"
                    >
                        <Icon name="heroicons:trash" size="12" />
                    </button>
                </div>

                <!-- Tags -->
                <MarkersMarkerTagEditor :marker-id="marker.id" />
            </div>
        </div>

        <!-- Marker count -->
        <div v-if="markers.length > 0" class="text-dim text-right text-[10px]">
            {{ markers.length }}/50 markers
        </div>

        <!-- Label autocomplete pickers (teleported to body) -->
        <WatchMarkerLabelPicker
            :visible="showNewMarkerSuggestions"
            :suggestions="filteredSuggestions"
            :anchor-el="newMarkerInputRef"
            :selected-index="selectedSuggestionIndex"
            @select="selectSuggestion"
            @close="showNewMarkerSuggestions = false"
        />
        <WatchMarkerLabelPicker
            :visible="showEditSuggestions"
            :suggestions="filteredSuggestions"
            :anchor-el="editInputRef?.[0] ?? null"
            :selected-index="selectedSuggestionIndex"
            @select="selectSuggestion"
            @close="showEditSuggestions = false"
        />

        <!-- Delete Confirmation Modal -->
        <Teleport to="body">
            <div
                v-if="showDeleteConfirm"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="cancelDelete"
            >
                <div
                    class="border-border bg-panel mx-4 w-full max-w-sm rounded-xl border p-5
                        shadow-2xl"
                >
                    <div class="mb-4 flex items-start gap-3">
                        <div class="bg-lava/10 flex items-center justify-center rounded-full p-2">
                            <Icon
                                name="heroicons:exclamation-triangle"
                                size="20"
                                class="text-lava"
                            />
                        </div>
                        <div>
                            <h3 class="text-sm font-medium text-white">Delete Marker</h3>
                            <p class="text-dim mt-1 text-xs">
                                Are you sure you want to delete this marker
                                <template v-if="markerToDelete?.label">
                                    "<span class="text-white">{{ markerToDelete.label }}</span
                                    >"</template
                                >? This action cannot be undone.
                            </p>
                        </div>
                    </div>
                    <div class="flex justify-end gap-2">
                        <button
                            class="border-border hover:bg-surface-hover rounded-lg border px-3
                                py-1.5 text-xs text-white transition-colors"
                            @click="cancelDelete"
                        >
                            Cancel
                        </button>
                        <button
                            class="bg-lava hover:bg-lava/80 rounded-lg px-3 py-1.5 text-xs
                                font-medium text-white transition-colors"
                            @click="confirmDelete"
                        >
                            Delete
                        </button>
                    </div>
                </div>
            </div>
        </Teleport>
    </div>
</template>
