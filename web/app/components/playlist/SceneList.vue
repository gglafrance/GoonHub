<script setup lang="ts">
import type { PlaylistSceneEntry } from '~/types/playlist';

const props = defineProps<{
    scenes: PlaylistSceneEntry[];
    isOwner: boolean;
}>();

const emit = defineEmits<{
    remove: [sceneId: number];
    removeMany: [sceneIds: number[]];
    reorder: [sceneIds: number[]];
    play: [index: number];
}>();

const { formatDuration } = useFormatter();

const selectMode = ref(false);
const selectedIds = ref<Set<number>>(new Set());

const allSelected = computed(
    () => props.scenes.length > 0 && selectedIds.value.size === props.scenes.length,
);

const toggleSelectMode = () => {
    selectMode.value = !selectMode.value;
    if (!selectMode.value) {
        selectedIds.value = new Set();
    }
};

const toggleSelect = (sceneId: number) => {
    const next = new Set(selectedIds.value);
    if (next.has(sceneId)) {
        next.delete(sceneId);
    } else {
        next.add(sceneId);
    }
    selectedIds.value = next;
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
                </span>
            </template>
        </div>

        <!-- Scene rows -->
        <div class="space-y-1">
            <div
                v-for="(entry, index) in scenes"
                :key="entry.scene.id"
                class="border-border bg-surface hover:bg-elevated group flex items-center gap-3
                    rounded-lg border p-2 transition-all"
                :class="
                    selectMode && selectedIds.has(entry.scene.id) ? 'border-lava/40 bg-lava/5' : ''
                "
                @click="selectMode ? toggleSelect(entry.scene.id) : undefined"
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
                <span v-else class="text-dim w-6 shrink-0 text-center font-mono text-[11px]">
                    {{ index + 1 }}
                </span>

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
                        <NuxtTime :datetime="entry.added_at" format="short" />
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
                    :style="{ marginBottom: 'max(0.75rem, env(safe-area-inset-bottom))' }"
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
