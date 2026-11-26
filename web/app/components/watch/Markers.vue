<script setup lang="ts">
import type { Marker, MarkerLabelSuggestion } from '~/types/marker';
import type { QueueItemInput } from '~/types/together';
import type { MarkerTimeField } from '~/utils/markerTime';
import type { MarkerListSort } from '~/utils/markerList';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const route = useRoute();
const authStore = useAuthStore();
const { createMarker, updateMarker, deleteMarker, fetchLabelSuggestions } = useApiMarkers();
const { fetchProcessingConfig } = useApiJobs();
const { formatMarkerRange } = useFormatter();
const { observe } = useAnimatedMarkerPreview();
const getPlayerTime = inject<() => number>('getPlayerTime');
const seekToTime = inject<(time: number) => void>('seekToTime');
const playMarker =
    inject<(marker: Pick<Marker, 'timestamp' | 'end_timestamp'>) => void>('playMarker');
const refreshMarkers = inject<() => Promise<void>>('refreshMarkers');

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

const sceneId = computed(() => parseInt(route.params.id as string));

// Use centralized data for markers and loading state
const loading = computed(() => watchPageData?.loading.player ?? true);
const markers = computed(() => watchPageData?.markers.value ?? []);
// Markers of other users (shared by an admin) are listed read-only: only the owner edits
const isOwn = (marker: Marker) => marker.user_id === authStore.user?.id;
// Creating and editing markers needs markers:create (own markers only)
const perms = useMyPermissions();
const canCreateMarkers = computed(() => perms.has('markers:create'));
const canEdit = (marker: Marker) => canCreateMarkers.value && isOwn(marker);
// "Watch together": queue the marker (its range and label) in a Goon2gether lobby
const canTogether = computed(() => perms.has('goon2gether:use'));
const togetherItems = ref<QueueItemInput[] | null>(null);
const watchTogether = (marker: Marker) => {
    togetherItems.value = [
        {
            scene_id: sceneId.value,
            start: marker.timestamp,
            end: marker.end_timestamp ?? null,
            label: marker.label || null,
        },
    ];
};
// Labels are server-wide: creating one needs marker_labels:manage (else pick an existing one)
const canCreateLabels = computed(() => perms.has('marker_labels:manage'));
// Every marker shown (own and shared), for "Add all to playlist"
const allMarkerIds = computed(() => markers.value.map((m) => m.id));
// The 50 markers limit counts the user's own markers only
const ownMarkerCount = computed(() => markers.value.filter(isOwn).length);
// List order (by time or grouped by label), remembered in this browser
const listSort = ref<MarkerListSort>('time');
const markerGroups = computed(() => markerListGroups(markers.value, listSort.value));
watch(listSort, (value) => {
    try {
        localStorage.setItem(MARKER_LIST_SORT_STORAGE_KEY, value);
    } catch {
        // Storage unavailable (private mode): the choice lasts for the page only
    }
});
// Scene duration in seconds: the API refuses a marker time past it
const sceneDuration = computed(() => watchPageData?.scene.value?.duration ?? null);

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

// Marker thumbnail type (static or animated)
const markerThumbnailType = ref('static');

// Autocomplete state
const labelSuggestions = ref<MarkerLabelSuggestion[]>([]);
const suggestionsLoaded = ref(false);
const showNewMarkerSuggestions = ref(false);
const showEditSuggestions = ref(false);
const selectedSuggestionIndex = ref(-1);

const loadSuggestions = async () => {
    if (suggestionsLoaded.value) return;
    try {
        const data = await fetchLabelSuggestions();
        labelSuggestions.value = data.labels || [];
        suggestionsLoaded.value = true;
    } catch {
        // Silently fail - autocomplete is a nice-to-have
    }
};

const filteredSuggestions = computed(() => {
    const query = showEditSuggestions.value ? editingLabel.value : newMarkerLabel.value;
    if (!query.trim()) return labelSuggestions.value;
    const lowerQuery = query.toLowerCase();
    return labelSuggestions.value.filter((s) => s.label.toLowerCase().includes(lowerQuery));
});

// What the typed label resolves to (existing, new for a label manager, or unknown)
const labelPick = computed(() =>
    pickMarkerLabel(
        showEditSuggestions.value ? editingLabel.value : newMarkerLabel.value,
        labelSuggestions.value,
        canCreateLabels.value,
    ),
);
const pickerCreateName = computed(() =>
    labelPick.value.kind === 'create' ? labelPick.value.name : null,
);
const pickerUnknownName = computed(() =>
    labelPick.value.kind === 'unknown' && suggestionsLoaded.value ? labelPick.value.name : null,
);

// The label to send: the canonical spelling of an existing label; null (with the error set)
// for a label that does not exist and that the user cannot create
const resolveTypedLabel = (typed: string): string | null => {
    const pick = pickMarkerLabel(typed, labelSuggestions.value, canCreateLabels.value);
    if (pick.kind === 'unknown' && suggestionsLoaded.value) {
        error.value = unknownMarkerLabelMessage(pick.name);
        return null;
    }
    return pick.kind === 'empty' ? '' : pick.name;
};

const createLabelFromPicker = (name: string) => {
    selectSuggestion({ label: name, count: 0 });
};

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

// New and edited markers show at once; their thumbnails are made in the background (remade
// when a marker moves) and polled for
const { insertMarker, waitForThumbnail, applyMarkerUpdate } = useNewMarkers({
    markers: watchPageData?.markers,
    refresh: refreshMarkers,
    hasThumbnail: (marker) =>
        markerThumbnailType.value === 'animated'
            ? !!marker.animated_thumbnail_path
            : !!marker.thumbnail_path,
});

// Start and end changes move the marker at once (list, timeline, scrubber) and are put back if
// the API refuses them
const { update: updateMarkerTimes } = useMarkerTimeUpdate({
    sceneId: () => sceneId.value,
    insertMarker,
    applyMarkerUpdate,
});

// Time typed on a card (click on its start or end)
const editingTime = ref<{ id: number; field: MarkerTimeField } | null>(null);
const timeSaving = ref(false);
const timeError = ref<string | null>(null);

const startEditingTime = (marker: Marker, field: MarkerTimeField) => {
    editingTime.value = { id: marker.id, field };
    timeError.value = null;
};

const cancelEditingTime = () => {
    editingTime.value = null;
    timeError.value = null;
};

const handleSaveTime = async (marker: Marker, field: MarkerTimeField, seconds: number) => {
    const current = field === 'start' ? marker.timestamp : marker.end_timestamp;
    if (seconds === current) {
        cancelEditingTime();
        return;
    }
    timeSaving.value = true;
    timeError.value = null;
    const result = await updateMarkerTimes(
        marker,
        field === 'start' ? { timestamp: seconds } : { end_timestamp: seconds },
        field === 'start' ? 'Failed to set the marker start' : 'Failed to set the marker end',
    );
    timeSaving.value = false;
    if (result.ok) {
        if (editingTime.value?.id === marker.id) cancelEditingTime();
    } else if (!('stale' in result)) {
        timeError.value = result.error;
    }
};

// Player time as a marker time (same time source as M, the stream offset included)
const playerMarkerTime = () => quickMarkerTimestamp(getPlayerTime?.() ?? 0);

const handleAddMarker = async () => {
    if (saving.value) return;

    const currentTime = getPlayerTime?.() ?? 0;
    error.value = null;
    const label = resolveTypedLabel(newMarkerLabel.value);
    if (label === null) return;
    saving.value = true;

    try {
        const marker = await createMarker(sceneId.value, {
            timestamp: Math.floor(currentTime),
            label: label || undefined,
        });
        newMarkerLabel.value = '';
        // Show the new marker at once (list and timeline), without waiting for a refetch
        insertMarker(marker);
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
        // Its thumbnail is made in the background: refetch until it is there
        waitForThumbnail(marker.id);
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to add marker';
    } finally {
        saving.value = false;
    }
};

const handleSeekToMarker = (marker: Marker) => {
    if (playMarker || seekToTime) {
        if (playMarker) {
            playMarker(marker);
        } else {
            seekToTime!(marker.timestamp);
        }
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
    const label = resolveTypedLabel(editingLabel.value);
    if (label === null) return;
    try {
        const updated = await updateMarker(sceneId.value, marker.id, { label });
        applyMarkerUpdate(marker, updated);
        cancelEditing();
        // Refresh parent markers for timeline indicators
        refreshMarkers?.();
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to update marker';
    }
};

// Sets the start of the marker to the current player time
const handleSetStart = async (marker: Marker, event: Event) => {
    event.stopPropagation();
    const start = playerMarkerTime();
    if (marker.end_timestamp != null && start >= marker.end_timestamp) {
        error.value = 'Seek before the marker end to set its start';
        return;
    }
    const invalid = validateMarkerStart(start, marker.end_timestamp, sceneDuration.value);
    if (invalid) {
        error.value = invalid;
        return;
    }
    error.value = null;
    const result = await updateMarkerTimes(
        marker,
        { timestamp: start },
        'Failed to set the marker start',
    );
    if (!result.ok && !('stale' in result)) error.value = result.error;
};

// Sets the end of the marker range to the current player time
const handleSetEnd = async (marker: Marker, event: Event) => {
    event.stopPropagation();
    const end = playerMarkerTime();
    if (end <= marker.timestamp) {
        error.value = 'Seek past the marker start to set its end';
        return;
    }
    const invalid = validateMarkerEnd(marker.timestamp, end, sceneDuration.value);
    if (invalid) {
        error.value = invalid;
        return;
    }
    error.value = null;
    const result = await updateMarkerTimes(
        marker,
        { end_timestamp: end },
        'Failed to set the marker end',
    );
    if (!result.ok && !('stale' in result)) error.value = result.error;
};

const handleClearEnd = async (marker: Marker, event: Event) => {
    event.stopPropagation();
    error.value = null;
    if (editingTime.value?.id === marker.id && editingTime.value.field === 'end') {
        cancelEditingTime();
    }
    const result = await updateMarkerTimes(
        marker,
        { clear_end_timestamp: true },
        'Failed to clear the marker end',
    );
    if (!result.ok && !('stale' in result)) error.value = result.error;
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
        await deleteMarker(sceneId.value, markerId);
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
    return markerThumbnailUrl(marker.id, marker.updated_at);
};

const getAnimatedUrl = (marker: Marker) => {
    if (!marker.animated_thumbnail_path) return null;
    return markerThumbnailUrl(marker.id, marker.updated_at, true);
};

onMounted(async () => {
    try {
        listSort.value = parseMarkerListSort(localStorage.getItem(MARKER_LIST_SORT_STORAGE_KEY));
    } catch {
        // Storage unavailable: keep the time order
    }
    // Load marker thumbnail type from processing config
    try {
        const config = await fetchProcessingConfig();
        markerThumbnailType.value = config.marker_thumbnail_type || 'static';
    } catch {
        // Default to static
    }
});
</script>

<template>
    <div class="space-y-4">
        <!-- Add Marker Form -->
        <div v-if="canCreateMarkers" class="flex items-center gap-2">
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
                title="Add marker at current time (M on the player: quick marker)"
                @click="handleAddMarker"
            >
                <Icon v-if="saving" name="svg-spinners:ring-resize" size="12" />
                <Icon v-else name="heroicons:plus" size="12" />
                Add Marker
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
            {{ canCreateMarkers ? 'No markers yet. Add one to bookmark a moment.' : 'No markers' }}
        </div>

        <!-- Markers Grid: one headerless group by time, or one group per label -->
        <div v-else class="space-y-4">
            <div class="flex items-center justify-between gap-2">
                <WatchMarkersSortToggle v-model="listSort" class="w-fit" />
                <MarkersPlaylistAddButton
                    :marker-ids="allMarkerIds"
                    :label="allMarkerIds.length === 1 ? 'Add to playlist' : 'Add all to playlist'"
                />
            </div>
            <section v-for="group in markerGroups" :key="group.key">
                <div v-if="listSort === 'label'" class="mb-2 flex items-center gap-2 text-[11px]">
                    <span
                        :class="group.label ? 'font-medium text-white' : 'text-dim italic'"
                        class="max-w-[70%] truncate"
                        :title="group.label ?? undefined"
                    >
                        {{ group.label ?? 'No label' }}
                    </span>
                    <span
                        class="border-border text-dim rounded-full border px-1.5 font-mono
                            text-[10px] leading-4"
                    >
                        {{ group.markers.length }}
                    </span>
                    <div class="bg-border h-px flex-1" />
                </div>
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                    <div
                        v-for="marker in group.markers"
                        :key="marker.id"
                        class="border-border bg-surface group relative overflow-hidden rounded-lg
                            border transition-all hover:border-white/20"
                    >
                        <!-- Thumbnail / Placeholder, with the start and end (click to edit) -->
                        <div class="relative">
                            <button
                                class="relative aspect-video w-full cursor-pointer overflow-hidden
                                    bg-black/40"
                                :title="
                                    markerHasRange(marker)
                                        ? `Play ${formatMarkerRange(marker.timestamp, marker.end_timestamp)}`
                                        : `Seek to ${formatMarkerTime(marker.timestamp)}`
                                "
                                @click="handleSeekToMarker(marker)"
                            >
                                <MarkersTagsBadge :tags="marker.tags" :label="marker.label" />
                                <!-- Animated Thumbnail -->
                                <video
                                    v-if="
                                        markerThumbnailType === 'animated' && getAnimatedUrl(marker)
                                    "
                                    :ref="(el) => observe(el as HTMLVideoElement)"
                                    :src="getAnimatedUrl(marker)!"
                                    muted
                                    loop
                                    playsinline
                                    preload="none"
                                    class="h-full w-full object-contain transition-transform
                                        duration-200 group-hover:scale-105"
                                />
                                <!-- Static Thumbnail Image -->
                                <img
                                    v-else-if="getThumbnailUrl(marker)"
                                    :src="getThumbnailUrl(marker)!"
                                    :alt="marker.label || 'Marker thumbnail'"
                                    class="h-full w-full object-cover transition-transform
                                        duration-200 group-hover:scale-105"
                                />
                                <!-- Placeholder when no thumbnail -->
                                <div
                                    v-else
                                    class="flex h-full w-full items-center justify-center
                                        bg-linear-to-br from-white/5 to-transparent"
                                >
                                    <Icon name="heroicons:photo" size="24" class="text-dim/40" />
                                </div>

                                <!-- Play Icon on Hover -->
                                <div
                                    class="bg-lava/25 absolute inset-0 flex items-center
                                        justify-center opacity-0 transition-opacity duration-200
                                        group-hover:opacity-100"
                                >
                                    <Icon
                                        name="heroicons:play-solid"
                                        size="28"
                                        class="text-white"
                                    />
                                </div>
                            </button>
                            <WatchMarkersTimeBadge
                                :marker="marker"
                                :editing="editingTime?.id === marker.id ? editingTime.field : null"
                                :readonly="!canEdit(marker)"
                                @edit="startEditingTime(marker, $event)"
                            />
                        </div>

                        <!-- Typed start or end -->
                        <WatchMarkersTimeEditor
                            v-if="editingTime?.id === marker.id"
                            :key="`${marker.id}-${editingTime.field}`"
                            :field="editingTime.field"
                            :marker="marker"
                            :duration="sceneDuration"
                            :saving="timeSaving"
                            :server-error="timeError"
                            @save="handleSaveTime(marker, editingTime.field, $event)"
                            @cancel="cancelEditingTime"
                        />

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
                                                class="border-border bg-panel w-full rounded-md
                                                    border px-2 py-1 text-[11px] text-white
                                                    focus:border-white/20 focus:outline-none"
                                                @focus="handleEditFocus"
                                                @blur="handleEditBlur(marker)"
                                                @keydown="handleSuggestionKeydown($event, true)"
                                                @keyup.enter="
                                                    selectedSuggestionIndex < 0 &&
                                                    handleSaveEdit(marker)
                                                "
                                                @keyup.escape="cancelEditing"
                                            />
                                        </div>
                                        <button
                                            class="text-emerald hover:bg-emerald/10 shrink-0 rounded
                                                p-1 transition-colors"
                                            title="Save"
                                            @click="handleSaveEdit(marker)"
                                        >
                                            <Icon name="heroicons:check" size="12" />
                                        </button>
                                        <button
                                            class="text-dim hover:bg-surface-hover shrink-0 rounded
                                                p-1 transition-colors"
                                            title="Cancel"
                                            @click="cancelEditing"
                                        >
                                            <Icon name="heroicons:x-mark" size="12" />
                                        </button>
                                    </div>
                                </template>
                                <div
                                    v-else-if="!canEdit(marker)"
                                    class="flex min-w-0 items-center gap-1"
                                    :title="
                                        isOwn(marker)
                                            ? 'Read-only'
                                            : 'Shared marker of another user (read-only)'
                                    "
                                >
                                    <Icon
                                        v-if="!isOwn(marker)"
                                        name="heroicons:user-group"
                                        size="11"
                                        class="text-dim/60 shrink-0"
                                    />
                                    <span
                                        class="line-clamp-1 text-[11px]"
                                        :class="marker.label ? 'text-dim' : 'text-dim/50 italic'"
                                    >
                                        {{ marker.label || 'No label' }}
                                    </span>
                                </div>
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

                            <!-- Range end: clear / set at current time -->
                            <button
                                v-if="
                                    canEdit(marker) &&
                                    editingMarkerId !== marker.id &&
                                    markerHasRange(marker)
                                "
                                class="text-dim hover:bg-surface-hover flex shrink-0 items-center
                                    justify-center rounded p-1.5 opacity-0 transition-all
                                    group-hover:opacity-100 hover:text-white"
                                title="Remove end (point marker)"
                                @click="handleClearEnd(marker, $event)"
                            >
                                <Icon name="heroicons:arrows-pointing-in" size="12" />
                            </button>
                            <!-- Start: set at current time (next to the end) -->
                            <button
                                v-if="canEdit(marker) && editingMarkerId !== marker.id"
                                class="text-dim hover:bg-surface-hover flex shrink-0 items-center
                                    justify-center rounded p-1.5 opacity-0 transition-all
                                    group-hover:opacity-100 hover:text-white"
                                title="Move start to current time"
                                aria-label="Set start"
                                @click="handleSetStart(marker, $event)"
                            >
                                <Icon name="heroicons:arrow-left-start-on-rectangle" size="12" />
                            </button>
                            <button
                                v-if="canEdit(marker) && editingMarkerId !== marker.id"
                                class="text-dim hover:bg-surface-hover flex shrink-0 items-center
                                    justify-center rounded p-1.5 opacity-0 transition-all
                                    group-hover:opacity-100 hover:text-white"
                                :title="
                                    markerHasRange(marker)
                                        ? 'Move end to current time'
                                        : 'Set end at current time (range marker)'
                                "
                                @click="handleSetEnd(marker, $event)"
                            >
                                <Icon name="heroicons:arrow-right-end-on-rectangle" size="12" />
                            </button>

                            <!-- Watch the marker in a Goon2gether lobby -->
                            <button
                                v-if="canTogether && editingMarkerId !== marker.id"
                                class="text-dim hover:bg-surface-hover flex shrink-0 items-center
                                    justify-center rounded p-1.5 opacity-0 transition-all
                                    group-hover:opacity-100 hover:text-white"
                                title="Watch together"
                                @click.stop="watchTogether(marker)"
                            >
                                <Icon name="heroicons:users" size="12" />
                            </button>
                            <!-- Add to a marker playlist -->
                            <MarkersPlaylistAddButton
                                v-if="editingMarkerId !== marker.id"
                                variant="row"
                                :marker-ids="[marker.id]"
                            />
                            <!-- Share the marker (send to friends, link to its range) -->
                            <ShareButton
                                v-if="editingMarkerId !== marker.id"
                                variant="row"
                                label="Share marker"
                                :subject="{
                                    kind: 'marker',
                                    markerId: marker.id,
                                    sceneId: marker.scene_id,
                                    label: marker.label,
                                    start: marker.timestamp,
                                    end: marker.end_timestamp ?? null,
                                }"
                            />

                            <!-- Delete Button -->
                            <button
                                v-if="canEdit(marker) && editingMarkerId !== marker.id"
                                class="text-dim hover:text-lava hover:bg-lava/10 flex shrink-0
                                    items-center justify-center rounded p-1.5 opacity-0
                                    transition-all group-hover:opacity-100"
                                title="Delete marker"
                                @click="handleDeleteClick(marker, $event)"
                            >
                                <Icon name="heroicons:trash" size="12" />
                            </button>
                        </div>

                        <!-- Tags -->
                        <MarkersMarkerTagEditor
                            :marker-id="marker.id"
                            :initial-tags="marker.tags"
                            :readonly="!canEdit(marker)"
                        />
                    </div>
                </div>
            </section>
        </div>

        <!-- Marker count -->
        <div v-if="ownMarkerCount > 0" class="text-dim text-right text-[10px]">
            {{ ownMarkerCount }}/50 markers
        </div>

        <!-- Label autocomplete pickers (teleported to body) -->
        <WatchMarkerLabelPicker
            :visible="showNewMarkerSuggestions"
            :suggestions="filteredSuggestions"
            :anchor-el="newMarkerInputRef"
            :selected-index="selectedSuggestionIndex"
            :create-name="pickerCreateName"
            :unknown-name="pickerUnknownName"
            @select="selectSuggestion"
            @create="createLabelFromPicker"
            @close="showNewMarkerSuggestions = false"
        />
        <WatchMarkerLabelPicker
            :visible="showEditSuggestions"
            :suggestions="filteredSuggestions"
            :anchor-el="editInputRef?.[0] ?? null"
            :selected-index="selectedSuggestionIndex"
            :create-name="pickerCreateName"
            :unknown-name="pickerUnknownName"
            @select="selectSuggestion"
            @create="createLabelFromPicker"
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
        <TogetherAddModal
            v-if="togetherItems"
            :visible="!!togetherItems"
            :items="togetherItems"
            @close="togetherItems = null"
        />
    </div>
</template>
