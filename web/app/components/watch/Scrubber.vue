<script setup lang="ts">
import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';

type Player = ReturnType<typeof videojs>;

// Thumbnail scrubber strip under the player (like Stash's scene scrubber): the sprite sheet
// thumbnails side by side, dragged or wheeled sideways, a click seeks to a thumbnail. Only the
// visible thumbnails are rendered, each one a tile of a sprite sheet (CSS background).
const props = defineProps<{
    player: Player | null;
    cues: VttCue[];
    markers: Marker[];
    // whether the scene has sprite sheets (false: not generated yet)
    hasSprites: boolean;
}>();

const emit = defineEmits<{
    seek: [time: number];
    playMarker: [marker: Marker];
    hide: [];
}>();

// The cues are only read: skip the reactive proxies of thousands of objects
const cueList = computed<VttCue[]>(() => toRaw(props.cues));
const count = computed(() => cueList.value.length);
const tileHeight = SCRUBBER_TILE_HEIGHT;
const tileWidth = computed(() => {
    const first = cueList.value[0];
    return first ? scrubberTileWidth(first.w, first.h, tileHeight) : 0;
});
const contentWidth = computed(() => count.value * tileWidth.value);

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

const scroll = useScrubberScroll(scroller, { contentWidth, onUserScroll: pauseFollow });

// Thumbnails to render (the visible ones plus a few on each side)
const range = computed(() =>
    visibleTileRange(scroll.scrollLeft.value, scroll.viewWidth.value, tileWidth.value, count.value),
);
const first = computed(() => range.value.first);
const last = computed(() => range.value.last);
const tiles = computed(() => {
    const out: { index: number; cue: VttCue; label: string }[] = [];
    for (let i = first.value; i <= last.value; i++) {
        const cue = cueList.value[i]!;
        out.push({ index: i, cue, label: formatScrubberTime(cue.start) });
    }
    return out;
});

const tileStyle = (index: number, cue: VttCue) => ({
    transform: `translateX(${index * tileWidth.value}px)`,
    width: `${tileWidth.value}px`,
    height: `${tileHeight}px`,
    '--sprite': `url("${cue.url}")`,
    '--sprite-x': `${-cue.x}px`,
    '--sprite-y': `${-cue.y}px`,
    '--sprite-w': `${cue.w}px`,
    '--sprite-h': `${cue.h}px`,
    '--sprite-scale': cue.h > 0 ? tileHeight / cue.h : 1,
});

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
    const index = findCueIndex(cueList.value, currentTime);
    if (index !== activeIndex.value) activeIndex.value = index;
    positionX = timeToOffset(cueList.value, currentTime, tileWidth.value);
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
watch([cueList, tileWidth], async () => {
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
    const index = tileIndexAt(offset, tileWidth.value, count.value);
    const cue = index >= 0 ? cueList.value[index] : undefined;
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
    const index = adjacentCueIndex(cueList.value, currentTime, e.key === 'ArrowLeft' ? -1 : 1);
    const cue = cueList.value[index];
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

const markerLaneHeight = computed(() => (props.markers.length ? 18 : 0));

onBeforeUnmount(() => {
    if (rafId) cancelAnimationFrame(rafId);
    const p = props.player;
    if (p) for (const [ev, fn] of playerEvents) p.off(ev, fn);
});
</script>

<template>
    <div class="scrubber border-border bg-surface/60 rounded-xl border">
        <div class="flex h-7 items-center gap-2 px-2.5">
            <Icon name="heroicons:film" size="13" class="text-lava" />
            <span class="text-[11px] font-medium text-white">Scrubber</span>
            <span v-if="count" class="text-dim font-mono text-[10px]">
                {{ count }} thumbnails
            </span>
            <span v-else-if="!hasSprites" class="text-dim text-[11px]">
                Thumbnails not generated yet
            </span>
            <span v-else class="text-dim text-[11px]">Loading thumbnails...</span>
            <div class="ml-auto flex items-center gap-1">
                <button
                    v-if="count && followPaused"
                    type="button"
                    class="border-lava/30 bg-lava/10 text-lava hover:bg-lava/20 flex h-5
                        items-center gap-1 rounded-md border px-1.5 text-[10px] font-medium
                        transition-colors"
                    title="Follow playback"
                    @click="resumeFollow"
                >
                    <Icon name="heroicons:arrows-pointing-in" size="11" />
                    Follow
                </button>
                <button
                    type="button"
                    class="text-dim flex h-5 w-5 items-center justify-center rounded-md
                        transition-colors hover:bg-white/5 hover:text-white"
                    title="Hide the scrubber"
                    aria-label="Hide the scrubber"
                    @click="emit('hide')"
                >
                    <Icon name="heroicons:x-mark" size="13" />
                </button>
            </div>
        </div>

        <div v-if="count" class="relative">
            <div
                ref="scroller"
                class="scrubber-scroller focus-visible:ring-lava/50 overflow-x-auto
                    overflow-y-hidden rounded-b-xl outline-none focus-visible:ring-1"
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
                        :cues="cueList"
                        :tile-width="tileWidth"
                        :lane-height="markerLaneHeight"
                        :tile-height="tileHeight"
                        @select="onMarker"
                    />
                    <div class="absolute inset-x-0 bottom-0" :style="{ height: `${tileHeight}px` }">
                        <div
                            v-for="tile in tiles"
                            :key="tile.index"
                            class="scrubber-tile"
                            :class="{ 'scrubber-tile--active': tile.index === activeIndex }"
                            :style="tileStyle(tile.index, tile.cue)"
                            :title="tile.label"
                        >
                            <div class="scrubber-sprite" />
                            <span class="scrubber-time">{{ tile.label }}</span>
                        </div>
                    </div>
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

.scrubber-tile {
    position: absolute;
    top: 0;
    left: 0;
    overflow: hidden;
    border-right: 1px solid rgba(5, 5, 5, 0.9);
    background: #0a0a0a;
    contain: strict;
}

.scrubber-sprite {
    width: var(--sprite-w);
    height: var(--sprite-h);
    background-image: var(--sprite);
    background-position: var(--sprite-x) var(--sprite-y);
    background-repeat: no-repeat;
    transform: scale(var(--sprite-scale));
    transform-origin: 0 0;
    opacity: 0.72;
    transition: opacity 0.12s ease;
    pointer-events: none;
}

.scrubber-tile:hover .scrubber-sprite {
    opacity: 1;
}

.scrubber-tile--active .scrubber-sprite {
    opacity: 1;
}

.scrubber-tile--active::after {
    content: '';
    position: absolute;
    inset: 0;
    border: 1px solid rgba(255, 77, 77, 0.9);
    box-shadow: inset 0 0 12px rgba(255, 77, 77, 0.25);
    pointer-events: none;
}

.scrubber-time {
    position: absolute;
    left: 3px;
    bottom: 3px;
    padding: 0 3px;
    border-radius: 3px;
    background: rgba(5, 5, 5, 0.7);
    color: rgba(255, 255, 255, 0.8);
    font-family: var(--font-mono);
    font-size: 9px;
    line-height: 13px;
    pointer-events: none;
}

.scrubber-tile--active .scrubber-time {
    color: #ff4d4d;
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
