<script setup lang="ts">
import type { PlaylistDetail, PlaylistSceneEntry } from '~/types/playlist';

const props = defineProps<{
    playlist: PlaylistDetail;
    currentIndex: number;
    effectiveOrder: number[];
    isShuffled: boolean;
}>();

const emit = defineEmits<{
    goToScene: [index: number];
    shuffle: [];
    unshuffle: [];
}>();

const { formatDuration } = useFormatter();

const getScene = (orderIndex: number): PlaylistSceneEntry | null => {
    const sceneIndex = props.effectiveOrder[orderIndex];
    if (sceneIndex === undefined) return null;
    return props.playlist.scenes[sceneIndex] ?? null;
};

const scrollContainer = ref<HTMLElement | null>(null);

// Auto-scroll to current scene
watch(
    () => props.currentIndex,
    async () => {
        await nextTick();
        const el = scrollContainer.value;
        if (!el) return;
        const active = el.querySelector('[data-active="true"]');
        if (active) {
            active.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
        }
    },
    { immediate: true },
);
</script>

<template>
    <div class="border-border bg-surface flex flex-col overflow-hidden rounded-xl border">
        <!-- Header -->
        <div class="border-border flex items-center justify-between border-b px-3 py-2.5">
            <div class="min-w-0">
                <NuxtLink
                    :to="`/playlists/${playlist.uuid}`"
                    class="block truncate text-xs font-semibold text-white hover:underline"
                >
                    {{ playlist.name }}
                </NuxtLink>
                <span class="text-dim font-mono text-[10px]">
                    {{ currentIndex + 1 }} of {{ effectiveOrder.length }}
                </span>
            </div>
            <button
                class="rounded p-1 transition-colors"
                :class="isShuffled ? 'text-lava' : 'text-dim hover:text-white'"
                :title="isShuffled ? 'Unshuffle' : 'Shuffle'"
                @click="isShuffled ? emit('unshuffle') : emit('shuffle')"
            >
                <Icon name="heroicons:arrows-right-left" size="16" />
            </button>
        </div>

        <!-- Scene list -->
        <div ref="scrollContainer" class="max-h-[50vh] overflow-y-auto">
            <button
                v-for="(_, orderIdx) in effectiveOrder"
                :key="orderIdx"
                :data-active="orderIdx === currentIndex"
                class="flex w-full items-center gap-2.5 px-2.5 py-2 text-left transition-colors"
                :class="
                    orderIdx === currentIndex
                        ? 'bg-lava/10 border-lava border-l-2'
                        : 'hover:bg-elevated border-l-2 border-transparent'
                "
                @click="emit('goToScene', orderIdx)"
            >
                <!-- Index -->
                <span
                    class="w-5 shrink-0 text-center font-mono text-[10px]"
                    :class="orderIdx === currentIndex ? 'text-lava font-semibold' : 'text-dim'"
                >
                    {{ orderIdx === currentIndex ? '>' : orderIdx + 1 }}
                </span>

                <!-- Thumbnail -->
                <div class="bg-void relative h-9 w-14 shrink-0 overflow-hidden rounded">
                    <img
                        v-if="getScene(orderIdx)?.scene.thumbnail_path"
                        :src="`/thumbnails/${getScene(orderIdx)!.scene.id}`"
                        class="h-full w-full object-cover"
                        :alt="getScene(orderIdx)!.scene.title"
                        loading="lazy"
                    />
                </div>

                <!-- Info -->
                <div class="min-w-0 flex-1">
                    <div
                        class="truncate text-[11px] font-medium"
                        :class="orderIdx === currentIndex ? 'text-white' : 'text-white/80'"
                    >
                        {{ getScene(orderIdx)?.scene.title }}
                    </div>
                    <div class="text-dim font-mono text-[9px]">
                        {{ formatDuration(getScene(orderIdx)?.scene.duration ?? 0) }}
                    </div>
                </div>
            </button>
        </div>
    </div>
</template>
