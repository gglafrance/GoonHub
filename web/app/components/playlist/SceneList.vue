<script setup lang="ts">
import type { PlaylistSceneEntry } from '~/types/playlist';
import type { SceneListItem } from '~/types/scene';

const props = defineProps<{
    scenes: PlaylistSceneEntry[];
    isOwner: boolean;
    playlistUuid: string;
}>();

const emit = defineEmits<{
    remove: [sceneId: number];
    removeMany: [sceneIds: number[]];
    reorder: [sceneIds: number[]];
    play: [index: number];
}>();

const { formatDuration, formatSize } = useFormatter();

// List or table (Settings > table columns; no rating nor actors in playlists)
const { isTable } = useSceneListView('playlist');
const tableScenes = computed(() => props.scenes.map((s) => s.scene));
const tableRowLink = (scene: SceneListItem, index: number) => ({
    path: `/watch/${scene.id}`,
    query: { playlist: props.playlistUuid, pos: String(index) },
});

const dragSelect = (ids: number[], additive: boolean) => {
    selectedIds.value = new Set(additive ? [...selectedIds.value, ...ids] : ids);
};

const selectMode = ref(false);
const selectedIds = ref<Set<number>>(new Set());

// Total disk size of the selection (every playlist scene is loaded, with its size)
// Sizes are omitted by the API without files:view_size: no total then
const perms = useMyPermissions();
const selectionSize = computed(() => {
    if (!perms.has('files:view_size')) return null;
    const sizes = new Map<number, number>();
    rememberSceneSizes(
        sizes,
        props.scenes.map((s) => s.scene),
    );
    return selectionSizeLabel(summarizeSelectionSize(selectedIds.value, sizes), formatSize);
});

const allSelected = computed(
    () => props.scenes.length > 0 && selectedIds.value.size === props.scenes.length,
);

const toggleSelectMode = () => {
    selectMode.value = !selectMode.value;
    if (!selectMode.value) {
        selectedIds.value = new Set();
    }
};

// Last toggled scene: a shift-click selects from it to the clicked scene
let selectionAnchor: number | null = null;
const toggleSelect = (sceneId: number, range = false) => {
    selectedIds.value = toggleSceneInSelection(
        selectedIds.value,
        props.scenes.map((s) => s.scene.id),
        sceneId,
        selectionAnchor,
        range,
    );
    selectionAnchor = sceneId;
};

// A row's hover checkbox (owner) turns select mode on, Escape clears the selection
useCardSelectMode(
    selectMode,
    computed(() => selectedIds.value.size > 0),
    () => {
        selectedIds.value = new Set();
        selectionAnchor = null;
    },
);

const onHoverCheckbox = (event: MouseEvent, sceneId: number) => {
    event.preventDefault();
    event.stopPropagation();
    toggleSelect(sceneId, event.shiftKey);
};

const toggleAll = () => {
    if (allSelected.value) {
        selectedIds.value = new Set();
    } else {
        selectedIds.value = new Set(props.scenes.map((s) => s.scene.id));
    }
};

const handleRemoveSelected = () => {
    if (selectedIds.value.size === 0) return;
    emit('removeMany', [...selectedIds.value]);
    selectedIds.value = new Set();
    selectMode.value = false;
};

const moveScene = (index: number, direction: 'up' | 'down') => {
    const ids = props.scenes.map((s) => s.scene.id);
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= ids.length) return;
    [ids[index], ids[targetIndex]] = [ids[targetIndex]!, ids[index]!];
    emit('reorder', ids);
};
</script>

<template>
    <div>
        <!-- Select mode header -->
        <div v-if="isOwner && scenes.length > 0" class="mb-2 flex items-center gap-2">
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
                    <template v-if="selectionSize">
                        &middot;
                        <span class="font-mono text-white/80">{{ selectionSize.size }}</span>
                        <template v-if="selectionSize.note"> ({{ selectionSize.note }}) </template>
                    </template>
                </span>
            </template>
        </div>

        <!-- Scene table -->
        <template v-if="isTable && scenes.length > 0">
            <SceneTable
                :scenes="tableScenes"
                :hidden-columns="['rating', 'actors']"
                :selectable="selectMode"
                :is-scene-selected="(id: number) => selectedIds.has(id)"
                :row-link="tableRowLink"
                show-position
                @toggle-selection="toggleSelect"
                @drag-select="dragSelect"
            >
                <template v-if="isOwner && !selectMode" #actions="{ scene, index }">
                    <div
                        class="flex items-center justify-end gap-0.5 opacity-0 transition-opacity
                            group-hover:opacity-100"
                    >
                        <button
                            :disabled="index === 0"
                            class="text-dim rounded p-1 transition-colors hover:text-white
                                disabled:opacity-30"
                            title="Move up"
                            @click="moveScene(index, 'up')"
                        >
                            <Icon name="heroicons:chevron-up" size="14" />
                        </button>
                        <button
                            :disabled="index === scenes.length - 1"
                            class="text-dim rounded p-1 transition-colors hover:text-white
                                disabled:opacity-30"
                            title="Move down"
                            @click="moveScene(index, 'down')"
                        >
                            <Icon name="heroicons:chevron-down" size="14" />
                        </button>
                        <button
                            class="text-dim hover:text-lava rounded p-1 transition-colors"
                            title="Remove from playlist"
                            @click="emit('remove', scene.id)"
                        >
                            <Icon name="heroicons:x-mark" size="14" />
                        </button>
                    </div>
                </template>
            </SceneTable>
        </template>

        <!-- Scene rows -->
        <div v-else class="space-y-1">
            <div
                v-for="(entry, index) in scenes"
                :key="entry.scene.id"
                class="border-border bg-surface hover:bg-elevated group flex items-center gap-3
                    rounded-lg border p-2 transition-all"
                :class="
                    selectMode && selectedIds.has(entry.scene.id) ? 'border-lava/40 bg-lava/5' : ''
                "
                @click="selectMode ? toggleSelect(entry.scene.id, $event.shiftKey) : undefined"
            >
                <!-- Checkbox (select mode) or Position number -->
                <div v-if="selectMode" class="flex w-6 shrink-0 items-center justify-center">
                    <div
                        class="flex h-4 w-4 items-center justify-center rounded border
                            transition-all"
                        :class="
                            selectedIds.has(entry.scene.id)
                                ? 'border-lava bg-lava'
                                : 'border-border bg-void'
                        "
                    >
                        <Icon
                            v-if="selectedIds.has(entry.scene.id)"
                            name="heroicons:check"
                            size="12"
                            class="text-white"
                        />
                    </div>
                </div>
                <div v-else class="relative flex w-6 shrink-0 items-center justify-center">
                    <span
                        class="text-dim font-mono text-[11px] transition-opacity"
                        :class="isOwner ? 'group-hover:opacity-0' : ''"
                    >
                        {{ index + 1 }}
                    </span>
                    <!-- Hover checkbox (pointer devices): starts a selection with this scene -->
                    <button
                        v-if="isOwner"
                        type="button"
                        class="border-border bg-void pointer-events-none absolute flex h-4 w-4
                            items-center justify-center rounded border opacity-0 transition-all
                            group-hover:pointer-events-auto group-hover:opacity-100
                            hover:border-white/50 focus-visible:pointer-events-auto
                            focus-visible:opacity-100"
                        aria-label="Select scene"
                        @mousedown="$event.shiftKey && $event.preventDefault()"
                        @click="onHoverCheckbox($event, entry.scene.id)"
                    />
                </div>

                <!-- Thumbnail -->
                <NuxtLink
                    :to="`/watch/${entry.scene.id}`"
                    class="bg-void relative h-12 w-20 shrink-0 overflow-hidden rounded"
                    :class="selectMode ? 'pointer-events-none' : ''"
                    @click.prevent="!selectMode && emit('play', index)"
                >
                    <img
                        v-if="entry.scene.thumbnail_path"
                        :src="`/thumbnails/${entry.scene.id}`"
                        class="h-full w-full object-cover"
                        :alt="entry.scene.title"
                        loading="lazy"
                    />
                    <div v-else class="flex h-full w-full items-center justify-center">
                        <Icon name="heroicons:play" size="16" class="text-dim" />
                    </div>
                    <!-- Duration -->
                    <div
                        v-if="entry.scene.duration > 0"
                        class="bg-void/90 absolute right-0.5 bottom-0.5 rounded px-1 py-px font-mono
                            text-[8px] text-white"
                    >
                        {{ formatDuration(entry.scene.duration) }}
                    </div>
                </NuxtLink>

                <!-- Title & meta -->
                <div class="min-w-0 flex-1">
                    <NuxtLink
                        :to="`/watch/${entry.scene.id}`"
                        class="block truncate text-xs font-medium text-white/90 transition-colors
                            hover:text-white"
                        :class="selectMode ? 'pointer-events-none' : ''"
                        :title="entry.scene.title"
                        @click.prevent="!selectMode && emit('play', index)"
                    >
                        {{ entry.scene.title }}
                    </NuxtLink>
                    <div class="text-dim mt-0.5 font-mono text-[10px]">
                        <UiFormattedDate :datetime="entry.added_at" />
                    </div>
                </div>

                <!-- Reorder & remove controls (owner only, not in select mode) -->
                <div
                    v-if="isOwner && !selectMode"
                    class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity
                        group-hover:opacity-100"
                >
                    <button
                        :disabled="index === 0"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        @click="moveScene(index, 'up')"
                    >
                        <Icon name="heroicons:chevron-up" size="14" />
                    </button>
                    <button
                        :disabled="index === scenes.length - 1"
                        class="text-dim rounded p-1 transition-colors hover:text-white
                            disabled:opacity-30"
                        @click="moveScene(index, 'down')"
                    >
                        <Icon name="heroicons:chevron-down" size="14" />
                    </button>
                    <button
                        class="text-dim hover:text-lava rounded p-1 transition-colors"
                        @click="emit('remove', entry.scene.id)"
                    >
                        <Icon name="heroicons:x-mark" size="14" />
                    </button>
                </div>
            </div>

            <div
                v-if="scenes.length === 0"
                class="border-border flex h-32 items-center justify-center rounded-lg border
                    border-dashed"
            >
                <p class="text-dim text-sm">No scenes in this playlist</p>
            </div>
        </div>

        <!-- Floating action bar for bulk removal -->
        <Teleport to="body">
            <Transition name="slide-up-action">
                <div
                    v-if="selectMode && selectedIds.size > 0"
                    class="glass-panel border-border fixed inset-x-0 bottom-0 z-40 mx-3 border"
                    :style="{ marginBottom: 'calc(0.75rem + var(--app-bottom-clear))' }"
                >
                    <div class="flex items-center justify-between px-4 py-3">
                        <span class="text-sm font-medium text-white">
                            {{ selectedIds.size }} scene{{ selectedIds.size !== 1 ? 's' : '' }}
                            selected
                        </span>
                        <button
                            class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg
                                px-4 py-2 text-xs font-semibold text-white transition-all"
                            @click="handleRemoveSelected"
                        >
                            <Icon name="heroicons:trash" size="14" />
                            Remove Selected
                        </button>
                    </div>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>

<style scoped>
.slide-up-action-enter-active,
.slide-up-action-leave-active {
    transition:
        transform 0.2s cubic-bezier(0.32, 0.72, 0, 1),
        opacity 0.2s ease;
}

.slide-up-action-enter-from,
.slide-up-action-leave-to {
    transform: translateY(100%);
    opacity: 0;
}
</style>
