<script setup lang="ts">
import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';

type Player = ReturnType<typeof videojs>;

// Centered mode of the scrubber strip (like Stash): a fixed line in the middle is the playback
// position, the strip moves under it while the video plays. Dragging the strip (mouse, touch) or
// scrolling it (wheel, trackpad, when the scrubber wheel setting is on) seeks the video to the time under the line, so the strip works
// as a fine-grained progress bar; a click on a thumbnail brings it to the centre.
const props = defineProps<{
    player: Player | null;
    cues: VttCue[];
    markers: Marker[];
    tileWidth: number;
    tileHeight: number;
    markerLaneHeight: number;
    // the player plays the file itself (no conversion): keyframe seeks are possible while dragging
    directStream: boolean;
}>();

const emit = defineEmits<{
    seek: [time: number];
    playMarker: [marker: Marker];
}>();

const count = computed(() => props.cues.length);
const contentWidth = computed(() => count.value * props.tileWidth);
const height = computed(() => props.markerLaneHeight + props.tileHeight);

const viewport = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const timeLabel = ref<HTMLElement | null>(null);

const viewWidth = ref(0);
// left edge of the visible content (for the rendered thumbnails), and the thumbnail at the centre
const viewLeft = ref(0);
const activeIndex = ref(-1);
const dragging = ref(false);

const range = computed(() =>
    visibleTileRange(viewLeft.value, viewWidth.value, props.tileWidth, count.value),
);
// Numbers: the tiles only render again when the range changes, not on every frame
const first = computed(() => range.value.first);
const last = computed(() => range.value.last);

// Content offset under the centre line
let offset = 0;
let label = '';

const seeker = useScrubberSeek({
    player: () => props.player,
    directStream: () => props.directStream,
});

const duration = () => {
    const d = props.player?.duration();
    return typeof d === 'number' && Number.isFinite(d) ? d : 0;
};

// Moves the content so that `offset` is under the centre line; the DOM is written directly
// (every frame while playing), the reactive state only when a thumbnail changes
const render = (time: number) => {
    if (content.value) {
        content.value.style.transform = `translateX(${centeredTranslate(offset, viewWidth.value)}px)`;
    }
    viewLeft.value = centeredViewLeft(offset, viewWidth.value);
    const index = findCueIndex(props.cues, time);
    if (index !== activeIndex.value) activeIndex.value = index;
    const text = formatScrubberPreciseTime(time);
    if (text !== label && timeLabel.value) {
        label = text;
        timeLabel.value.textContent = text;
    }
};

// Whether the strip is being moved by the user (the player time lags behind the strip then)
let wheelTimer: ReturnType<typeof setTimeout> | null = null;
const interacting = () => dragging.value || wheelTimer !== null || seeker.active();

// Follows the playback position
const sync = () => {
    const p = props.player;
    if (!p || count.value === 0 || interacting()) return;
    const time = p.currentTime() ?? 0;
    offset = centerOffsetForTime(props.cues, time, props.tileWidth);
    render(time);
};

// Smooth movement while playing; timeupdate keeps it going where rAF is throttled
let playing = false;
let rafId = 0;
const loop = () => {
    sync();
    rafId = playing ? requestAnimationFrame(loop) : 0;
};
const onPlay = () => {
    playing = true;
    if (!rafId) rafId = requestAnimationFrame(loop);
};
const onPause = () => {
    playing = false;
    sync();
};

const playerEvents: [string, () => void][] = [
    ['timeupdate', sync],
    ['seeking', sync],
    ['seeked', sync],
    ['loadedmetadata', sync],
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
        playing = !p.paused();
        if (playing) onPlay();
        sync();
    },
    { immediate: true },
);

// The strip moved to `offset`: show it and seek there (throttled)
const moveTo = (next: number) => {
    offset = clampCenterOffset(next, contentWidth.value);
    const time = centerTime(props.cues, offset, props.tileWidth, duration());
    render(time);
    seeker.request(time);
};

// End of a drag or wheel scroll: precise seek to the time under the line
const settle = () => {
    const time = centerTime(props.cues, offset, props.tileWidth, duration());
    seeker.finish(time);
    emit('seek', time);
};

// Drag (mouse, touch, pen). Touch keeps the vertical page scroll (touch-action: pan-y).
let pressed = false;
let pointerId = -1;
let startX = 0;
let startOffset = 0;
let resumeAfterDrag = false;
// a drag just ended: the click that follows must not seek
let suppressClick = false;

const onPointerDown = (e: PointerEvent) => {
    if (e.button !== 0 || !props.player || count.value === 0) return;
    pressed = true;
    pointerId = e.pointerId;
    startX = e.clientX;
    startOffset = offset;
};

const startDrag = (e: PointerEvent) => {
    dragging.value = true;
    viewport.value?.setPointerCapture(e.pointerId);
    // Like Stash: the video pauses while dragged and plays again on release
    const p = props.player;
    resumeAfterDrag = !!p && !p.paused();
    if (resumeAfterDrag) p?.pause();
};

const onPointerMove = (e: PointerEvent) => {
    if (!pressed || e.pointerId !== pointerId) return;
    if (!dragging.value) {
        if (Math.abs(e.clientX - startX) < SCRUBBER_DRAG_THRESHOLD) return;
        startDrag(e);
    }
    e.preventDefault();
    moveTo(dragCenterOffset(startOffset, startX, e.clientX, contentWidth.value));
};

const onPointerUp = (e: PointerEvent) => {
    if (e.pointerId !== pointerId) return;
    pressed = false;
    pointerId = -1;
    if (!dragging.value) return;
    dragging.value = false;
    suppressClick = e.type === 'pointerup';
    // the click (if any) is dispatched right after pointerup
    setTimeout(() => {
        suppressClick = false;
    }, 0);
    if (viewport.value?.hasPointerCapture(e.pointerId)) {
        viewport.value.releasePointerCapture(e.pointerId);
    }
    settle();
    if (resumeAfterDrag) props.player?.play()?.catch(() => {});
    resumeAfterDrag = false;
    sync();
};

const consumeDrag = () => {
    const was = suppressClick;
    suppressClick = false;
    return was;
};

// Wheel and trackpad, when the scrubber wheel setting is on: the dominant axis moves the strip
// (playback goes on); off, the wheel is left to the page
const settingsStore = useSettingsStore();
const onWheel = (e: WheelEvent) => {
    if (dragging.value || !props.player) return;
    const delta = scrubberWheelDelta(e, settingsStore.scrubberWheel, viewWidth.value);
    if (delta === 0) return;
    // At an end, let the page scroll instead of trapping the wheel
    if ((offset <= 0 && delta < 0) || (offset >= contentWidth.value && delta > 0)) return;
    e.preventDefault();
    if (wheelTimer) clearTimeout(wheelTimer);
    wheelTimer = setTimeout(() => {
        wheelTimer = null;
        settle();
        sync();
    }, SCRUBBER_WHEEL_IDLE_MS);
    moveTo(offset + delta);
};

// Seeks through the player API like the progress bar (a converted stream reloads there)
const seek = (time: number) => {
    const p = props.player;
    if (!p) return;
    p.currentTime(time);
    emit('seek', time);
    sync();
};

const onContentClick = (e: MouseEvent) => {
    if (consumeDrag() || !content.value) return;
    const x = e.clientX - content.value.getBoundingClientRect().left;
    const index = tileIndexAt(x, props.tileWidth, count.value);
    const cue = index >= 0 ? props.cues[index] : undefined;
    if (cue) seek(cue.start);
};

const onMarker = (marker: Marker) => {
    if (consumeDrag()) return;
    emit('playMarker', marker);
};

// Left/right arrows: previous/next thumbnail (the player shortcuts are skipped)
const onKeydown = (e: KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    e.stopPropagation();
    const now = props.player?.currentTime() ?? 0;
    const index = adjacentCueIndex(props.cues, now, e.key === 'ArrowLeft' ? -1 : 1);
    const cue = props.cues[index];
    if (cue) seek(cue.start);
};

let resizeObserver: ResizeObserver | null = null;

onMounted(async () => {
    const node = viewport.value;
    if (node) {
        node.addEventListener('wheel', onWheel, { passive: false });
        viewWidth.value = node.clientWidth;
        resizeObserver = new ResizeObserver(() => {
            viewWidth.value = node.clientWidth;
            render(centerTime(props.cues, offset, props.tileWidth, duration()));
        });
        resizeObserver.observe(node);
    }
    await nextTick();
    sync();
});

// New thumbnails (scene change, first load): back to the playback position
watch([() => props.cues, () => props.tileWidth], async () => {
    activeIndex.value = -1;
    await nextTick();
    sync();
});

onBeforeUnmount(() => {
    if (rafId) cancelAnimationFrame(rafId);
    if (wheelTimer) clearTimeout(wheelTimer);
    viewport.value?.removeEventListener('wheel', onWheel);
    resizeObserver?.disconnect();
    const p = props.player;
    if (p) for (const [ev, fn] of playerEvents) p.off(ev, fn);
});
</script>

<template>
    <div
        ref="viewport"
        class="scrubber-centered focus-visible:ring-lava/50 relative overflow-hidden rounded-b-xl
            outline-none focus-visible:ring-1"
        :class="{ 'scrubber-centered--dragging': dragging }"
        :style="{ height: `${height}px` }"
        tabindex="0"
        role="slider"
        aria-label="Thumbnail scrubber, centered on the playback position"
        :aria-valuemin="0"
        :aria-valuemax="count - 1"
        :aria-valuenow="activeIndex"
        @keydown="onKeydown"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
    >
        <div
            ref="content"
            class="scrubber-centered-content absolute top-0 left-0"
            :style="{ width: `${contentWidth}px`, height: `${height}px` }"
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
        </div>
        <div
            class="scrubber-centered-line"
            :style="{ top: `${markerLaneHeight}px` }"
            aria-hidden="true"
        />
        <span
            ref="timeLabel"
            class="scrubber-centered-time"
            :style="{ top: `${markerLaneHeight + 3}px` }"
            aria-hidden="true"
        />
    </div>
</template>

<style scoped>
.scrubber-centered {
    user-select: none;
    -webkit-user-select: none;
    cursor: grab;
    touch-action: pan-y;
    background: rgba(5, 5, 5, 0.5);
}

.scrubber-centered--dragging {
    cursor: grabbing;
}

.scrubber-centered-content {
    will-change: transform;
}

.scrubber-centered-line {
    position: absolute;
    left: calc(50% - 1px);
    bottom: 0;
    width: 2px;
    background: #ff4d4d;
    box-shadow: 0 0 6px rgba(255, 77, 77, 0.8);
    pointer-events: none;
    z-index: 2;
}

.scrubber-centered-time {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    padding: 0 4px;
    border: 1px solid rgba(255, 77, 77, 0.5);
    border-radius: 4px;
    background: rgba(5, 5, 5, 0.85);
    color: #ff4d4d;
    font-family: var(--font-mono);
    font-size: 9px;
    line-height: 13px;
    pointer-events: none;
    z-index: 3;
}

.scrubber-centered-time:empty {
    display: none;
}
</style>
