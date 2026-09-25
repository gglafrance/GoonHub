<script setup lang="ts">
import type { ShortsFeed } from '~/composables/useShortsFeed';

// The vertical player. The page owns the list (shared with the grid) and sets
// activeIndex to the short to open on.
const props = defineProps<{
    feed: ShortsFeed;
}>();

const emit = defineEmits<{
    close: [];
}>();

const root = ref<HTMLElement | null>(null);
const scroller = ref<HTMLElement | null>(null);

const feed = props.feed;
const { items, activeIndex, loading, error, done, maxDuration } = feed;

// The info panel stays open across shorts and shows the one in view.
const infoOpen = ref(false);
const activeItem = computed(() => items.value[activeIndex.value] ?? null);

// One sound setting for the whole feed: unmuting once keeps the next shorts
// playing with sound.
const muted = ref(false);

// Maximize: real fullscreen where the browser allows it, otherwise (iOS) a
// fixed overlay.
const maximized = ref(false);
const overlayMaximize = ref(false);

const realign = () => {
    nextTick(() => {
        const slide = scroller.value?.querySelector<HTMLElement>(
            `[data-index="${activeIndex.value}"]`,
        );
        slide?.scrollIntoView({ block: 'start' });
    });
};

const enterMaximize = async () => {
    const el = root.value;
    if (el?.requestFullscreen) {
        try {
            await el.requestFullscreen();
            return;
        } catch {
            // Fall through to the overlay
        }
    }
    overlayMaximize.value = true;
    maximized.value = true;
    realign();
};

const exitMaximize = async () => {
    if (document.fullscreenElement) {
        try {
            await document.exitFullscreen();
        } catch {
            // Already left
        }
    }
    overlayMaximize.value = false;
    maximized.value = false;
    realign();
};

const toggleMaximize = () => (maximized.value ? exitMaximize() : enterMaximize());

const onFullscreenChange = () => {
    maximized.value = !!document.fullscreenElement || overlayMaximize.value;
    realign();
};

// Seconds per Left/Right press, as in the scene player
const SEEK_STEP = 5;

// Slide components by index, to control the active one from the keyboard
interface SlideHandle {
    togglePlay: () => void;
    seekBy: (delta: number) => void;
}
const slides = new Map<number, SlideHandle>();
const setSlideRef = (index: number, el: unknown) => {
    if (el) slides.set(index, el as SlideHandle);
    else slides.delete(index);
};

useKeyboardShortcuts([
    { key: 'arrowdown', action: () => feed.next(), description: 'Next short' },
    { key: 'arrowup', action: () => feed.prev(), description: 'Previous short' },
    {
        key: 'arrowleft',
        action: () => slides.get(activeIndex.value)?.seekBy(-SEEK_STEP),
        description: 'Back 5 seconds',
    },
    {
        key: 'arrowright',
        action: () => slides.get(activeIndex.value)?.seekBy(SEEK_STEP),
        description: 'Forward 5 seconds',
    },
    {
        key: ' ',
        action: () => slides.get(activeIndex.value)?.togglePlay(),
        description: 'Play/pause',
    },
    { key: 'm', action: () => (muted.value = !muted.value), description: 'Mute' },
    { key: 'i', action: () => (infoOpen.value = !infoOpen.value), description: 'Video info' },
    { key: 'f', action: toggleMaximize, description: 'Maximize' },
    {
        key: 'escape',
        action: () => {
            if (infoOpen.value) infoOpen.value = false;
            else if (maximized.value) exitMaximize();
            else emit('close');
        },
        description: 'Close info, leave maximize, then back to the grid',
    },
]);

onMounted(() => {
    document.addEventListener('fullscreenchange', onFullscreenChange);
    scroller.value?.addEventListener('wheel', feed.onWheel, { passive: false });
    feed.container.value = scroller.value;
    feed.jumpTo(activeIndex.value);
});

onBeforeUnmount(() => {
    document.removeEventListener('fullscreenchange', onFullscreenChange);
    scroller.value?.removeEventListener('wheel', feed.onWheel);
    feed.container.value = null;
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
});

const isLoaded = (i: number) => Math.abs(i - activeIndex.value) <= 1;
</script>

<template>
    <div
        ref="root"
        class="shorts-feed relative bg-black"
        :class="
            overlayMaximize
                ? 'fixed inset-0 z-[60]'
                : maximized
                  ? 'h-full w-full'
                  : 'border-border h-full w-full overflow-hidden rounded-xl border'
        "
    >
        <div
            ref="scroller"
            class="no-scrollbar h-full w-full snap-y snap-mandatory overflow-y-scroll
                overscroll-contain"
        >
            <div
                v-for="(item, i) in items"
                :key="item.id"
                :ref="(el) => feed.observe(el as Element | null)"
                :data-index="i"
                class="h-full w-full snap-start snap-always"
            >
                <ShortsItem
                    :ref="(el) => setSlideRef(i, el)"
                    v-model:muted="muted"
                    :item="item"
                    :active="i === activeIndex"
                    :loaded="isLoaded(i)"
                    :preload-next="i === activeIndex + 1"
                    :liked="feed.likes.value[item.id] ?? false"
                    :rating="feed.ratings.value[item.id] ?? 0"
                    :jizz-count="feed.jizzCounts.value[item.id] ?? 0"
                    :maximized="maximized"
                    :info-open="infoOpen"
                    @toggle-maximize="toggleMaximize"
                    @toggle-info="infoOpen = !infoOpen"
                />
            </div>

            <!-- Loading / end / empty -->
            <div
                v-if="loading && items.length === 0"
                class="flex h-full items-center justify-center"
            >
                <LoadingSpinner label="Loading shorts..." />
            </div>
            <div
                v-else-if="!loading && items.length === 0 && !error"
                class="flex h-full flex-col items-center justify-center gap-2 px-6 text-center"
            >
                <Icon name="heroicons:film" size="32" class="text-dim" />
                <p class="text-sm font-semibold text-white">No shorts yet</p>
                <p class="text-dim max-w-sm text-xs">
                    Scenes up to {{ maxDuration }} seconds long show up here. Cut one from a longer
                    scene with + Add short in its Shorts tab.
                </p>
            </div>
            <div
                v-else-if="done && items.length > 0"
                class="text-dim flex h-24 snap-start items-center justify-center text-[11px]"
            >
                That's all the shorts.
            </div>
        </div>

        <Transition
            enter-active-class="transition-transform duration-200 ease-out"
            enter-from-class="translate-x-full"
            leave-active-class="transition-transform duration-150 ease-in"
            leave-to-class="translate-x-full"
        >
            <ShortsInfoPanel
                v-if="infoOpen && activeItem"
                :scene-id="activeItem.id"
                @close="infoOpen = false"
            />
        </Transition>

        <div
            v-if="error"
            class="border-lava/30 bg-lava/10 absolute top-3 left-1/2 -translate-x-1/2 rounded-lg
                border px-3 py-1.5 text-xs text-white backdrop-blur-md"
        >
            {{ error }}
        </div>
    </div>
</template>

<style scoped>
.no-scrollbar {
    scrollbar-width: none;
}

.no-scrollbar::-webkit-scrollbar {
    display: none;
}

.shorts-feed:fullscreen {
    width: 100vw;
    height: 100vh;
    border: none;
    border-radius: 0;
}
</style>
