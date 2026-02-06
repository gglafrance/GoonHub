<script setup lang="ts">
import type { PlaylistSceneEntry } from '~/types/playlist';

const props = defineProps<{
    scenes: PlaylistSceneEntry[];
    isOwner: boolean;
}>();

const emit = defineEmits<{
    remove: [sceneId: number];
    reorder: [sceneIds: number[]];
    play: [index: number];
}>();

const { formatDuration } = useFormatter();

const moveScene = (index: number, direction: 'up' | 'down') => {
    const ids = props.scenes.map((s) => s.scene.id);
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= ids.length) return;
    [ids[index], ids[targetIndex]] = [ids[targetIndex]!, ids[index]!];
    emit('reorder', ids);
};
</script>

<template>
    <div class="space-y-1">
        <div
            v-for="(entry, index) in scenes"
            :key="entry.scene.id"
            class="border-border bg-surface hover:bg-elevated group flex items-center gap-3
                rounded-lg border p-2 transition-all"
        >
            <!-- Position number -->
            <span class="text-dim w-6 shrink-0 text-center font-mono text-[11px]">
                {{ index + 1 }}
            </span>

            <!-- Thumbnail -->
            <NuxtLink
                :to="`/watch/${entry.scene.id}`"
                class="bg-void relative h-12 w-20 shrink-0 overflow-hidden rounded"
                @click.prevent="emit('play', index)"
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
                    :title="entry.scene.title"
                    @click.prevent="emit('play', index)"
                >
                    {{ entry.scene.title }}
                </NuxtLink>
                <div class="text-dim mt-0.5 font-mono text-[10px]">
                    <NuxtTime :datetime="entry.added_at" format="short" />
                </div>
            </div>

            <!-- Reorder & remove controls (owner only) -->
            <div
                v-if="isOwner"
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
</template>
