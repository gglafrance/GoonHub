<script setup lang="ts">
import type { PlaylistDetail, PlaylistSceneEntry } from '~/types/playlist';

const props = defineProps<{
    playlist: PlaylistDetail;
    currentIndex: number;
    effectiveOrder: number[];
    isShuffled: boolean;
    hasNext: boolean;
    hasPrevious: boolean;
    countdownVisible: boolean;
}>();

const emit = defineEmits<{
    goToScene: [index: number];
    next: [];
    previous: [];
    shuffle: [];
    unshuffle: [];
}>();

const { formatDuration } = useFormatter();

const remainingDuration = computed(() => {
    let total = 0;
    for (let i = props.currentIndex + 1; i < props.effectiveOrder.length; i++) {
        const sceneIndex = props.effectiveOrder[i];
        if (sceneIndex === undefined) continue;
        const entry = props.playlist.scenes[sceneIndex];
        if (entry) {
            total += entry.scene.duration ?? 0;
        }
    }
    return total;
});

const expanded = ref(false);
const scrollContainer = ref<HTMLElement | null>(null);

const currentScene = computed(() => {
    const sceneIndex = props.effectiveOrder[props.currentIndex];
    if (sceneIndex === undefined) return null;
    return props.playlist.scenes[sceneIndex] ?? null;
});

const getScene = (orderIndex: number): PlaylistSceneEntry | null => {
    const sceneIndex = props.effectiveOrder[orderIndex];
    if (sceneIndex === undefined) return null;
    return props.playlist.scenes[sceneIndex] ?? null;
};

const openDrawer = () => {
    expanded.value = true;
};

const closeDrawer = () => {
    expanded.value = false;
};

const handleSceneSelect = (orderIndex: number) => {
    emit('goToScene', orderIndex);
    closeDrawer();
};

// Auto-scroll to active scene when drawer expands
watch(expanded, async (isExpanded) => {
    if (isExpanded) {
        document.body.style.overflow = 'hidden';
        await nextTick();
        const el = scrollContainer.value;
        if (!el) return;
        const active = el.querySelector('[data-active="true"]');
        if (active) {
            active.scrollIntoView({ block: 'center', behavior: 'instant' });
        }
    } else {
        document.body.style.overflow = '';
    }
});

// Auto-collapse when countdown starts
watch(
    () => props.countdownVisible,
    (visible) => {
        if (visible) {
            expanded.value = false;
        }
    },
);

onUnmounted(() => {
    document.body.style.overflow = '';
});
</script>

<template>
    <!-- Collapsed Mini-Bar -->
    <Transition name="minibar-slide">
        <div
            v-if="!countdownVisible && !expanded"
            class="glass-panel border-border fixed inset-x-0 bottom-0 z-40 mx-3 border xl:hidden"
            :style="{ marginBottom: 'max(0.75rem, env(safe-area-inset-bottom))' }"
        >
            <div class="flex items-center gap-3 px-3 py-2.5" @click="openDrawer">
                <!-- Current scene thumbnail -->
                <div class="bg-void relative h-9 w-14 shrink-0 overflow-hidden rounded">
                    <img
                        v-if="currentScene?.scene.thumbnail_path"
                        :src="`/thumbnails/${currentScene.scene.id}`"
                        class="h-full w-full object-cover"
                        :alt="currentScene.scene.title"
                    />
                </div>

                <!-- Playlist info -->
                <div class="min-w-0 flex-1">
                    <div class="truncate text-xs font-semibold text-white">
                        {{ playlist.name }}
                    </div>
                    <span class="text-dim font-mono text-[10px]">
                        {{ currentIndex + 1 }} of {{ effectiveOrder.length }}
                        <template v-if="remainingDuration > 0">
                            &middot; {{ formatDuration(remainingDuration) }} left
                        </template>
                    </span>
                </div>

                <!-- Controls (stop click propagation so tapping buttons doesn't open drawer) -->
                <div class="flex shrink-0 items-center gap-1" @click.stop>
                    <button
                        class="flex h-8 w-8 items-center justify-center rounded-full
                            transition-colors"
                        :class="
                            hasPrevious
                                ? 'text-white/80 hover:bg-white/10 hover:text-white'
                                : 'cursor-not-allowed text-white/20'
                        "
                        :disabled="!hasPrevious"
                        @click="emit('previous')"
                    >
                        <Icon name="heroicons:backward" size="16" />
                    </button>
                    <button
                        class="flex h-8 w-8 items-center justify-center rounded-full
                            transition-colors"
                        :class="
                            hasNext
                                ? 'text-white/80 hover:bg-white/10 hover:text-white'
                                : 'cursor-not-allowed text-white/20'
                        "
                        :disabled="!hasNext"
                        @click="emit('next')"
                    >
                        <Icon name="heroicons:forward" size="16" />
                    </button>
                    <button
                        class="flex h-8 w-8 items-center justify-center rounded-full
                            transition-colors"
                        :class="
                            isShuffled ? 'text-lava' : 'text-dim hover:bg-white/10 hover:text-white'
                        "
                        @click="isShuffled ? emit('unshuffle') : emit('shuffle')"
                    >
                        <Icon name="heroicons:arrows-right-left" size="14" />
                    </button>
                </div>
            </div>
        </div>
    </Transition>

    <!-- Expanded Drawer -->
    <Teleport to="body">
        <!-- Backdrop -->
        <Transition name="fade">
            <div
                v-if="expanded"
                class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm xl:hidden"
                @click="closeDrawer"
            />
        </Transition>

        <!-- Sheet -->
        <Transition name="slide-up">
            <div
                v-if="expanded"
                class="bg-surface fixed inset-x-0 bottom-0 z-50 flex max-h-[75vh] flex-col
                    rounded-t-2xl xl:hidden"
            >
                <!-- Drag handle -->
                <div class="flex justify-center py-3">
                    <div class="h-1 w-10 rounded-full bg-white/20" />
                </div>

                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 pb-3">
                    <div class="min-w-0 flex-1">
                        <NuxtLink
                            :to="`/playlists/${playlist.uuid}`"
                            class="block truncate text-sm font-semibold text-white hover:underline"
                        >
                            {{ playlist.name }}
                        </NuxtLink>
                        <span class="text-dim font-mono text-[10px]">
                            {{ currentIndex + 1 }} of {{ effectiveOrder.length }}
                            <template v-if="remainingDuration > 0">
                                &middot; {{ formatDuration(remainingDuration) }} left
                            </template>
                        </span>
                    </div>
                    <div class="flex items-center gap-1">
                        <button
                            class="flex h-8 w-8 items-center justify-center rounded-full
                                transition-colors"
                            :class="
                                isShuffled
                                    ? 'text-lava'
                                    : 'text-dim hover:bg-white/10 hover:text-white'
                            "
                            :title="isShuffled ? 'Unshuffle' : 'Shuffle'"
                            @click="isShuffled ? emit('unshuffle') : emit('shuffle')"
                        >
                            <Icon name="heroicons:arrows-right-left" size="16" />
                        </button>
                        <button
                            class="text-dim flex h-8 w-8 items-center justify-center rounded-full
                                transition-colors hover:bg-white/10 hover:text-white"
                            @click="closeDrawer"
                        >
                            <Icon name="heroicons:x-mark" size="20" />
                        </button>
                    </div>
                </div>

                <!-- Scene list -->
                <div
                    ref="scrollContainer"
                    class="flex-1 overflow-y-auto"
                    :style="{ paddingBottom: 'max(0.5rem, env(safe-area-inset-bottom))' }"
                >
                    <button
                        v-for="(_, orderIdx) in effectiveOrder"
                        :key="orderIdx"
                        :data-active="orderIdx === currentIndex"
                        class="flex w-full items-center gap-2.5 px-3 py-2.5 text-left
                            transition-colors"
                        :class="
                            orderIdx === currentIndex
                                ? 'bg-lava/10 border-lava border-l-2'
                                : 'hover:bg-elevated border-l-2 border-transparent'
                        "
                        @click="handleSceneSelect(orderIdx)"
                    >
                        <!-- Index -->
                        <span
                            class="w-5 shrink-0 text-center font-mono text-[10px]"
                            :class="
                                orderIdx === currentIndex ? 'text-lava font-semibold' : 'text-dim'
                            "
                        >
                            {{ orderIdx === currentIndex ? '>' : orderIdx + 1 }}
                        </span>

                        <!-- Thumbnail -->
                        <div class="bg-void relative h-10 w-16 shrink-0 overflow-hidden rounded">
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
        </Transition>
    </Teleport>
</template>

<style scoped>
.minibar-slide-enter-active,
.minibar-slide-leave-active {
    transition:
        transform 0.3s cubic-bezier(0.32, 0.72, 0, 1),
        opacity 0.3s cubic-bezier(0.32, 0.72, 0, 1);
}

.minibar-slide-enter-from,
.minibar-slide-leave-to {
    transform: translateY(100%);
    opacity: 0;
}

.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
    transition: transform 0.3s cubic-bezier(0.32, 0.72, 0, 1);
}

.slide-up-enter-from,
.slide-up-leave-to {
    transform: translateY(100%);
}
</style>
