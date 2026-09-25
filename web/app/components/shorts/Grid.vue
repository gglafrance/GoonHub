<script setup lang="ts">
import type { ShortsFeed } from '~/composables/useShortsFeed';

// Thumbnails of the shorts, in feed order. Clicking one opens the player there.
const props = defineProps<{
    feed: ShortsFeed;
    // Tile to bring into view on mount (the short last shown in the player)
    focusIndex: number | null;
}>();

const emit = defineEmits<{
    open: [index: number];
}>();

const { items, loading, error, done, maxDuration } = props.feed;

// Load the next page when the end of the grid comes near.
const LOAD_AHEAD_PX = 600;
const sentinel = ref<HTMLElement | null>(null);
let observer: IntersectionObserver | null = null;

// The observer only reports the end coming into view. When a page does not
// fill the screen the end never leaves it, so check again after each load.
const fillScreen = () => {
    nextTick(() => {
        const { loading: busy, done: finished, error: failed } = props.feed;
        if (busy.value || finished.value || failed.value || !sentinel.value) return;
        if (sentinel.value.getBoundingClientRect().top < window.innerHeight + LOAD_AHEAD_PX) {
            props.feed.loadMore();
        }
    });
};

watch(loading, (busy) => {
    if (!busy) fillScreen();
});

onMounted(() => {
    observer = new IntersectionObserver(
        (entries) => {
            if (entries.some((e) => e.isIntersecting)) props.feed.loadMore();
        },
        { rootMargin: `${LOAD_AHEAD_PX}px` },
    );
    if (sentinel.value) observer.observe(sentinel.value);
    fillScreen();

    if (props.focusIndex !== null) {
        nextTick(() => {
            document
                .querySelector<HTMLElement>(`[data-grid-index="${props.focusIndex}"]`)
                ?.scrollIntoView({ block: 'center' });
        });
    }
});

onBeforeUnmount(() => {
    observer?.disconnect();
    observer = null;
});
</script>

<template>
    <div>
        <div
            v-if="items.length > 0"
            class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5
                xl:grid-cols-6"
        >
            <ShortsGridTile
                v-for="(item, i) in items"
                :key="item.id"
                :data-grid-index="i"
                :item="item"
                :focused="i === focusIndex"
                @click="emit('open', i)"
            />
        </div>

        <div v-if="loading" class="flex justify-center py-8">
            <LoadingSpinner label="Loading shorts..." />
        </div>
        <div
            v-else-if="items.length === 0 && !error"
            class="flex flex-col items-center justify-center gap-2 px-6 py-20 text-center"
        >
            <Icon name="heroicons:film" size="32" class="text-dim" />
            <p class="text-sm font-semibold text-white">No shorts yet</p>
            <p class="text-dim max-w-sm text-xs">
                Scenes up to {{ maxDuration }} seconds long show up here. Cut one from a longer
                scene with + Add short in its Shorts tab.
            </p>
        </div>
        <div v-else-if="done && items.length > 0" class="text-dim py-6 text-center text-[11px]">
            That's all the shorts.
        </div>
        <div
            v-if="error"
            class="border-lava/30 bg-lava/10 mx-auto my-4 w-fit rounded-lg border px-3 py-1.5
                text-xs text-white"
        >
            {{ error }}
        </div>

        <div ref="sentinel" class="h-px" />
    </div>
</template>
