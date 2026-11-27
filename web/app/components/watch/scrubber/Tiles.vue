<script setup lang="ts">
import type { VttCue } from '~/composables/useVttParser';

// Thumbnails of the scrubber strip, from `first` to `last` (the visible ones): each one a tile of
// a sprite sheet (CSS background), placed at its index in the strip content.
const props = defineProps<{
    cues: VttCue[];
    first: number;
    last: number;
    tileWidth: number;
    tileHeight: number;
    activeIndex: number;
}>();

const tiles = computed(() => {
    const out: { index: number; cue: VttCue; label: string }[] = [];
    for (let i = props.first; i <= props.last; i++) {
        const cue = props.cues[i];
        if (cue) out.push({ index: i, cue, label: formatScrubberTime(cue.start) });
    }
    return out;
});

const tileStyle = (index: number, cue: VttCue) => ({
    transform: `translateX(${index * props.tileWidth}px)`,
    width: `${props.tileWidth}px`,
    height: `${props.tileHeight}px`,
    '--sprite': `url("${cue.url}")`,
    '--sprite-x': `${-cue.x}px`,
    '--sprite-y': `${-cue.y}px`,
    '--sprite-w': `${cue.w}px`,
    '--sprite-h': `${cue.h}px`,
    '--sprite-scale': cue.h > 0 ? props.tileHeight / cue.h : 1,
});
</script>

<template>
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
</template>

<style scoped>
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
</style>
