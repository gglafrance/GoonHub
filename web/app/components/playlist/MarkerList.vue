<script setup lang="ts">
import type { PlaylistMarkerEntry } from '~/types/playlist';

// Markers of a marker playlist: play one, and for the owner move, remove, select and remove
const props = defineProps<{
    markers: PlaylistMarkerEntry[];
    isOwner: boolean;
}>();

const emit = defineEmits<{
    remove: [markerId: number];
    removeMany: [markerIds: number[]];
    reorder: [markerIds: number[]];
    play: [index: number];
}>();

const { formatMarkerRange } = useFormatter();

const selectMode = ref(false);
const selectedIds = ref<Set<number>>(new Set());

const toggleSelectMode = () => {
    selectMode.value = !selectMode.value;
    if (!selectMode.value) selectedIds.value = new Set();
};

const toggleSelect = (markerId: number) => {
    const next = new Set(selectedIds.value);
    if (next.has(markerId)) next.delete(markerId);
    else next.add(markerId);
    selectedIds.value = next;
};

const allSelected = computed(
    () => props.markers.length > 0 && selectedIds.value.size === props.markers.length,
);

const toggleAll = () => {
    selectedIds.value = allSelected.value
        ? new Set()
        : new Set(props.markers.map((m) => m.marker.id));
};

const handleRemoveSelected = () => {
    if (selectedIds.value.size === 0) return;
    emit('removeMany', [...selectedIds.value]);
    selectedIds.value = new Set();
    selectMode.value = false;
};

const moveMarker = (index: number, direction: 'up' | 'down' | 'top' | 'bottom') => {
    const ids = props.markers.map((m) => m.marker.id);
    const [id] = ids.splice(index, 1);
    if (id === undefined) return;
    const target =
        direction === 'up'
            ? index - 1
            : direction === 'down'
              ? index + 1
              : direction === 'top'
                ? 0
                : ids.length;
    if (target < 0 || target > ids.length) return;
    ids.splice(target, 0, id);
    emit('reorder', ids);
};

const onRowClick = (entry: PlaylistMarkerEntry, index: number) => {
    if (selectMode.value) toggleSelect(entry.marker.id);
    else emit('play', index);
};
</script>

<template>
    <div>
        <!-- Select mode header -->
        <div v-if="isOwner && markers.length > 0" class="mb-2 flex items-center gap-2">
            <button
                class="text-dim rounded-md border px-2.5 py-1 text-[11px] font-medium
                    transition-all"
                :class="
                    selectMode
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : 'border-border bg-surface hover:text-white'
                "
                @click="toggleSelectMode"
            >
                <Icon
                    :name="selectMode ? 'heroicons:x-mark' : 'heroicons:check-circle'"
                    size="13"
                    class="mr-1 inline-block align-[-2px]"
                />
                {{ selectMode ? 'Cancel' : 'Select' }}
            </button>
            <template v-if="selectMode">
                <button
                    class="text-dim border-border bg-surface rounded-md border px-2.5 py-1
                        text-[11px] font-medium transition-all hover:text-white"
                    @click="toggleAll"
                >
                    {{ allSelected ? 'Deselect All' : 'Select All' }}
                </button>
                <span v-if="selectedIds.size > 0" class="text-dim text-[11px]">
                    {{ selectedIds.size }} selected
                </span>
            </template>
        </div>

        <div class="space-y-1">
            <div
                v-for="(entry, index) in markers"
                :key="entry.marker.id"
                class="border-border bg-surface hover:bg-elevated group flex cursor-pointer
                    items-center gap-3 rounded-lg border p-2 transition-all"
                :class="
                    selectMode && selectedIds.has(entry.marker.id) ? 'border-lava/40 bg-lava/5' : ''
                "
                @click="onRowClick(entry, index)"
            >
                <!-- Checkbox (select mode) or position -->
                <div class="flex w-6 shrink-0 items-center justify-center">
                    <div
                        v-if="selectMode"
                        class="flex h-4 w-4 items-center justify-center rounded border
                            transition-all"
                        :class="
                            selectedIds.has(entry.marker.id)
                                ? 'border-lava bg-lava'
                                : 'border-border bg-void'
                        "
                    >
                        <Icon
                            v-if="selectedIds.has(entry.marker.id)"
                            name="heroicons:check"
                            size="12"
                            class="text-white"
                        />
                    </div>
                    <span v-else class="text-dim font-mono text-[11px]">{{ index + 1 }}</span>
                </div>

                <!-- Marker thumbnail (versioned URL) -->
                <div class="bg-void relative h-12 w-20 shrink-0 overflow-hidden rounded">
                    <img
                        v-if="entry.marker.has_thumbnail"
                        :src="markerThumbnailUrl(entry.marker.id, entry.marker.thumbnail_version)"
                        class="h-full w-full object-cover"
                        :alt="entry.marker.label || entry.scene.title"
                        loading="lazy"
                    />
                    <div v-else class="flex h-full w-full items-center justify-center">
                        <Icon name="heroicons:bookmark" size="16" class="text-dim" />
                    </div>
                    <div
                        class="bg-void/90 absolute right-0.5 bottom-0.5 rounded px-1 py-px font-mono
                            text-[8px] text-white"
                    >
                        {{ formatMarkerRange(entry.start, entry.end) }}
                    </div>
                </div>

                <!-- Label, scene -->
                <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5">
                        <span
                            class="h-2 w-2 shrink-0 rounded-full"
                            :style="{ backgroundColor: entry.marker.color || '#FFFFFF' }"
                        />
                        <span
                            class="truncate text-xs font-medium text-white/90"
                            :title="entry.marker.label"
                        >
                            {{ entry.marker.label || 'Marker' }}
                        </span>
                        <span
                            v-if="entry.marker.end_timestamp === null"
                            class="text-dim shrink-0 text-[9px]"
                            title="No end: plays for the default duration"
                        >
                            auto end
                        </span>
                    </div>
                    <div class="text-dim mt-0.5 truncate text-[10px]" :title="entry.scene.title">
                        {{ entry.scene.title }}
                    </div>
                </div>

                <!-- Reorder & remove (owner, not in select mode) -->
                <div
                    v-if="isOwner && !selectMode"
                    class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity
                        group-hover:opacity-100"
                    @click.stop
                >
                    <button
                        :disabled="index === 0"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        title="Move to top"
                        @click="moveMarker(index, 'top')"
                    >
                        <Icon name="heroicons:chevron-double-up" size="14" />
                    </button>
                    <button
                        :disabled="index === 0"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        title="Move up"
                        @click="moveMarker(index, 'up')"
                    >
                        <Icon name="heroicons:chevron-up" size="14" />
                    </button>
                    <button
                        :disabled="index === markers.length - 1"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        title="Move down"
                        @click="moveMarker(index, 'down')"
                    >
                        <Icon name="heroicons:chevron-down" size="14" />
                    </button>
                    <button
                        :disabled="index === markers.length - 1"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        title="Move to bottom"
                        @click="moveMarker(index, 'bottom')"
                    >
                        <Icon name="heroicons:chevron-double-down" size="14" />
                    </button>
                    <button
                        class="text-dim hover:text-lava rounded p-1 transition-colors"
                        title="Remove from playlist"
                        @click="emit('remove', entry.marker.id)"
                    >
                        <Icon name="heroicons:x-mark" size="14" />
                    </button>
                </div>
            </div>

            <div
                v-if="markers.length === 0"
                class="border-border flex h-32 flex-col items-center justify-center rounded-lg
                    border border-dashed"
            >
                <p class="text-dim text-sm">No markers in this playlist</p>
                <p class="text-dim mt-1 text-[11px]">
                    Add markers from the Markers pages or the Markers tab of a scene
                </p>
            </div>
        </div>

        <!-- Floating action bar for bulk removal -->
        <Teleport to="body">
            <div
                v-if="selectMode && selectedIds.size > 0"
                class="glass-panel border-border fixed inset-x-0 bottom-0 z-40 mx-3 border"
                :style="{ marginBottom: 'max(0.75rem, env(safe-area-inset-bottom))' }"
            >
                <div class="flex items-center justify-between px-4 py-3">
                    <span class="text-sm font-medium text-white">
                        {{ selectedIds.size }} marker{{ selectedIds.size !== 1 ? 's' : '' }}
                        selected
                    </span>
                    <button
                        class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4
                            py-2 text-xs font-semibold text-white transition-all"
                        @click="handleRemoveSelected"
                    >
                        <Icon name="heroicons:trash" size="14" />
                        Remove Selected
                    </button>
                </div>
            </div>
        </Teleport>
    </div>
</template>
