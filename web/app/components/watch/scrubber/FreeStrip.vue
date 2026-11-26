<script setup lang="ts">
import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';

type Player = ReturnType<typeof videojs>;

// Free mode of the scrubber strip: dragged with inertia or (scrubber wheel setting on) wheeled
// sideways on its own, a click
// seeks to a thumbnail, a red line marks the playback position, and the strip follows playback
// (paused while the user scrolls it, resumed after a few seconds or by the Follow button).
const props = defineProps<{
    player: Player | null;
    cues: VttCue[];
    markers: Marker[];
    tileWidth: number;
    tileHeight: number;
    markerLaneHeight: number;
}>();

const emit = defineEmits<{
    seek: [time: number];
    playMarker: [marker: Marker];
}>();

const count = computed(() => props.cues.length);
const contentWidth = computed(() => count.value * props.tileWidth);

const scroller = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const indicator = ref<HTMLElement | null>(null);

// Following playback: paused when the user scrolls the strip, resumed a few seconds after
// (while playing), by the follow button, or by a seek from the strip
const followPaused = ref(false);
let lastUserScroll = 0;
const pauseFollow = () => {
    followPaused.value = true;
    lastUserScroll = performance.now();
};

const settingsStore = useSettingsStore();
const scroll = useScrubberScroll(scroller, {
    contentWidth,
    onUserScroll: pauseFollow,
    wheelEnabled: () => settingsStore.scrubberWheel,
});

// Thumbnails to render (the visible ones plus a few on each side)
const range = computed(() =>
    visibleTileRange(scroll.scrollLeft.value, scroll.viewWidth.value, props.tileWidth, count.value),
);
// Numbers: the tiles only render again when the range changes, not on every scroll event
const first = computed(() => range.value.first);
const last = computed(() => range.value.last);

const canScrollLeft = computed(() => scroll.scrollLeft.value > 1);
const canScrollRight = computed(
    () => scroll.scrollLeft.value < maxScroll(scroll.viewWidth.value, contentWidth.value) - 1,
);

// Playback position
const activeIndex = ref(-1);
const playing = ref(false);
let currentTime = 0;
let positionX = 0;

const followTo = (force: boolean) => {
    const view = scroll.viewWidth.value;
    if (!view || !contentWidth.value) return;
    const targetLeft = force
        ? positionX - view * SCRUBBER_FOLLOW_LEAD
        : followScrollTarget(positionX, scroll.scrollLeft.value, view, contentWidth.value);
    if (targetLeft !== null) scroll.scrollTo(targetLeft);
};

const tick = () => {
    const p = props.player;
    if (!p || count.value === 0) return;
    currentTime = p.currentTime() ?? 0;
    const index = findCueIndex(props.cues, currentTime);
    if (index !== activeIndex.value) activeIndex.value = index;
    positionX = timeToOffset(props.cues, currentTime, props.tileWidth);
    if (indicator.value) indicator.value.style.transform = `translateX(${positionX}px)`;
    if (followPaused.value) {
        const idle = performance.now() - lastUserScroll;
        if (!shouldResumeFollow(idle, playing.value, scroll.isBusy())) return;
        followPaused.value = false;
    }
    followTo(false);
};

// Smooth position while playing; timeupdate keeps it going where rAF is throttled
let rafId = 0;
const loop = () => {
    tick();
    rafId = playing.value ? requestAnimationFrame(loop) : 0;
};
const onPlay = () => {
    playing.value = true;
    if (!rafId) rafId = requestAnimationFrame(loop);
};
const onPause = () => {
    playing.value = false;
    tick();
};

const playerEvents: [string, () => void][] = [
    ['timeupdate', tick],
    ['seeking', tick],
    ['seeked', tick],
    ['loadedmetadata', tick],
    ['play', onPlay],
    ['playing', onPlay],
    ['pause', onPause],
    ['ended', onPause],
];

watch(
    () => props.player,
    (p, old) => {
        if (old) for (const [ev, fn] of playerEvents) old.off(ev, fn);
        if (!p) return;
        for (const [ev, fn] of playerEvents) p.on(ev, fn);
        playing.value = !p.paused();
        if (playing.value) onPlay();
        tick();
    },
    { immediate: true },
);

// Shows the playback position at once (no smooth scroll)
const jumpToPosition = () => {
    followPaused.value = false;
    tick();
    scroll.scrollTo(positionX - scroll.viewWidth.value * SCRUBBER_FOLLOW_LEAD, false);
};

// New thumbnails (scene change, first load): jump to the playback position
watch([() => props.cues, () => props.tileWidth], async () => {
    activeIndex.value = -1;
    await nextTick();
    jumpToPosition();
});

onMounted(async () => {
    await nextTick();
    jumpToPosition();
});

// Seeks through the player API like the progress bar (a converted stream reloads there)
const seek = (time: number) => {
    const p = props.player;
    if (!p) return;
    followPaused.value = false;
    p.currentTime(time);
    emit('seek', time);
    tick();
};

const onContentClick = (e: MouseEvent) => {
    if (scroll.consumeDrag() || !content.value) return;
    const offset = e.clientX - content.value.getBoundingClientRect().left;
    const index = tileIndexAt(offset, props.tileWidth, count.value);
    const cue = index >= 0 ? props.cues[index] : undefined;
    if (cue) seek(cue.start);
};

const onMarker = (marker: Marker) => {
    if (scroll.consumeDrag()) return;
    followPaused.value = false;
    emit('playMarker', marker);
};

// Left/right arrows: previous/next thumbnail (the player shortcuts are skipped)
const onKeydown = (e: KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    e.stopPropagation();
    const index = adjacentCueIndex(props.cues, currentTime, e.key === 'ArrowLeft' ? -1 : 1);
    const cue = props.cues[index];
    if (cue) seek(cue.start);
};

const page = (direction: -1 | 1) => {
    pauseFollow();
    scroll.scrollTo(
        pageScrollTarget(
            scroll.scrollLeft.value,
            scroll.viewWidth.value,
            contentWidth.value,
            direction,
        ),
        true,
        true,
    );
};

const resumeFollow = () => {
    followPaused.value = false;
    tick();
    followTo(true);
};

onBeforeUnmount(() => {
    if (rafId) cancelAnimationFrame(rafId);
    const p = props.player;
    if (p) for (const [ev, fn] of playerEvents) p.off(ev, fn);
});

defineExpose({ followPaused, resumeFollow });
</script>

<template>
    <div class="relative">
        <div
            ref="scroller"
            class="scrubber-scroller focus-visible:ring-lava/50 overflow-x-auto overflow-y-hidden
                rounded-b-xl outline-none focus-visible:ring-1"
            :class="{ 'scrubber-scroller--dragging': scroll.dragging.value }"
            tabindex="0"
            role="slider"
            aria-label="Thumbnail scrubber"
            :aria-valuemin="0"
            :aria-valuemax="count - 1"
            :aria-valuenow="activeIndex"
            @keydown="onKeydown"
        >
            <div
                ref="content"
                class="relative"
                :style="{
                    width: `${contentWidth}px`,
                    height: `${markerLaneHeight + tileHeight}px`,
                }"
                @click="onContentClick"
            >
                <WatchScrubberMarkers
                    v-if="markers.length"
                    :markers="markers"
                    :cues="cues"
                    :tile-width="tileWidth"
                    :lane-height="markerLaneHeight"
                    :tile-height="tileHeight"
                    @select="onMarker"
                />
                <WatchScrubberTiles
                    :cues="cues"
                    :first="first"
                    :last="last"
                    :tile-width="tileWidth"
                    :tile-height="tileHeight"
                    :active-index="activeIndex"
                />
                <div
                    ref="indicator"
                    class="scrubber-indicator"
                    :style="{ top: `${markerLaneHeight}px` }"
                    aria-hidden="true"
                />
            </div>
        </div>

        <button
            v-show="canScrollLeft"
            type="button"
            class="scrubber-page scrubber-page--left"
            :style="{ top: `${markerLaneHeight}px` }"
            aria-label="Previous thumbnails"
            @click="page(-1)"
        >
            <Icon name="heroicons:chevron-left" size="14" />
        </button>
        <button
            v-show="canScrollRight"
            type="button"
            class="scrubber-page scrubber-page--right"
            :style="{ top: `${markerLaneHeight}px` }"
            aria-label="Next thumbnails"
            @click="page(1)"
        >
            <Icon name="heroicons:chevron-right" size="14" />
        </button>
    </div>
</template>

<style scoped>
.scrubber-scroller {
    scrollbar-width: none;
    user-select: none;
    -webkit-user-select: none;
    cursor: grab;
    overscroll-behavior-x: contain;
    touch-action: pan-x pan-y;
}

.scrubber-scroller--dragging {
    cursor: grabbing;
}

.scrubber-scroller::-webkit-scrollbar {
    display: none;
}

.scrubber-indicator {
    position: absolute;
    left: -1px;
    bottom: 0;
    width: 2px;
    background: #ff4d4d;
    box-shadow: 0 0 6px rgba(255, 77, 77, 0.8);
    pointer-events: none;
    will-change: transform;
    z-index: 2;
}

.scrubber-page {
    position: absolute;
    bottom: 0;
    z-index: 3;
    display: flex;
    width: 22px;
    align-items: center;
    justify-content: center;
    color: rgba(255, 255, 255, 0.7);
    transition: color 0.12s ease;
}

.scrubber-page:hover {
    color: #ff4d4d;
}

.scrubber-page--left {
    left: 0;
    border-bottom-left-radius: 12px;
    background: linear-gradient(to right, rgba(5, 5, 5, 0.9), rgba(5, 5, 5, 0));
}

.scrubber-page--right {
    right: 0;
    border-bottom-right-radius: 12px;
    background: linear-gradient(to left, rgba(5, 5, 5, 0.9), rgba(5, 5, 5, 0));
}
</style>
