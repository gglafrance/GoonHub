<script setup lang="ts">
import type { VideoListItem } from '~/types/video';
import type { WatchProgress } from '~/types/homepage';

defineProps<{
    videos: VideoListItem[];
    watchProgress?: Record<number, WatchProgress>;
    ratings?: Record<number, number>;
}>();

const scrollContainer = ref<HTMLElement | null>(null);
const canScrollLeft = ref(false);
const canScrollRight = ref(true);

function updateScrollState() {
    if (!scrollContainer.value) return;
    const el = scrollContainer.value;
    canScrollLeft.value = el.scrollLeft > 0;
    canScrollRight.value = el.scrollLeft < el.scrollWidth - el.clientWidth - 1;
}

function scroll(direction: 'left' | 'right') {
    if (!scrollContainer.value) return;
    const scrollAmount = 320 * 3; // Scroll 3 cards (VideoCard width)
    scrollContainer.value.scrollBy({
        left: direction === 'left' ? -scrollAmount : scrollAmount,
        behavior: 'smooth',
    });
}

onMounted(() => {
    updateScrollState();
});
</script>

<template>
    <div class="group/carousel relative">
        <!-- Left arrow -->
        <button
            v-if="canScrollLeft"
            @click="scroll('left')"
            class="from-background/95 to-background/0 absolute top-0 bottom-0 left-0 z-30 flex w-12
                cursor-pointer items-center justify-start bg-linear-to-r pl-1 opacity-0
                transition-opacity group-hover/carousel:opacity-100"
        >
            <div
                class="bg-surface/90 border-border hover:bg-elevated flex h-8 w-8 items-center
                    justify-center rounded-full border backdrop-blur-sm transition-colors"
            >
                <Icon name="heroicons:chevron-left" size="18" class="text-white" />
            </div>
        </button>

        <!-- Horizontal scroll container -->
        <div
            ref="scrollContainer"
            @scroll="updateScrollState"
            class="scrollbar-hide -mx-4 flex gap-4 overflow-x-auto px-4 pb-2"
        >
            <div v-for="video in videos" :key="video.id" class="shrink-0">
                <VideoCard
                    :video="video"
                    :progress="watchProgress?.[video.id]"
                    :rating="ratings?.[video.id]"
                />
            </div>
        </div>

        <!-- Right arrow -->
        <button
            v-if="canScrollRight"
            @click="scroll('right')"
            class="from-background/95 to-background/0 absolute top-0 right-0 bottom-0 z-30 flex w-12
                cursor-pointer items-center justify-end bg-linear-to-l pr-1 opacity-0
                transition-opacity group-hover/carousel:opacity-100"
        >
            <div
                class="bg-surface/90 border-border hover:bg-elevated flex h-8 w-8 items-center
                    justify-center rounded-full border backdrop-blur-sm transition-colors"
            >
                <Icon name="heroicons:chevron-right" size="18" class="text-white" />
            </div>
        </button>
    </div>
</template>

<style scoped>
.scrollbar-hide {
    -ms-overflow-style: none;
    scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
    display: none;
}
</style>
