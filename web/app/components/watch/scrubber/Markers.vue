<script setup lang="ts">
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';

// Markers of the scrubber strip: a labelled chip in the lane above the thumbnails, a tick
// across them at the marker time, and a bar under them for the range of a range marker.
const props = defineProps<{
    markers: Marker[];
    cues: VttCue[];
    tileWidth: number;
    laneHeight: number;
    tileHeight: number;
}>();

const emit = defineEmits<{ select: [marker: Marker] }>();

const items = computed(() => {
    const placed = props.markers.map((marker) => {
        const x = timeToOffset(props.cues, marker.timestamp, props.tileWidth);
        const end = markerHasRange(marker)
            ? timeToOffset(props.cues, marker.end_timestamp, props.tileWidth)
            : null;
        const label = marker.label || formatScrubberTime(marker.timestamp);
        return {
            marker,
            x,
            rangeWidth: end !== null ? Math.max(2, end - x) : 0,
            label,
            width: markerChipWidth(label),
            color: marker.color || '#FF4D4D',
        };
    });
    const chipX = layoutChips(placed, props.cues.length * props.tileWidth);
    return placed.map((item, i) => ({ ...item, chipX: chipX[i] ?? item.x }));
});
</script>

<template>
    <div class="pointer-events-none absolute inset-0 z-[1]">
        <template v-for="item in items" :key="item.marker.id">
            <div
                class="absolute bottom-0 w-px opacity-70"
                :style="{
                    left: `${item.x}px`,
                    top: `${laneHeight}px`,
                    background: item.color,
                }"
            />
            <div
                v-if="item.rangeWidth"
                class="absolute bottom-0 h-[3px] opacity-80"
                :style="{
                    left: `${item.x}px`,
                    width: `${item.rangeWidth}px`,
                    background: item.color,
                }"
            />
            <button
                type="button"
                class="pointer-events-auto absolute top-0.5 flex h-[15px] items-center gap-1
                    overflow-hidden rounded-[4px] border border-white/10 bg-black/60 px-1 text-[9px]
                    leading-none whitespace-nowrap text-white/80 transition-colors
                    hover:border-white/25 hover:text-white"
                :style="{ left: `${item.chipX}px`, maxWidth: `${item.width}px` }"
                :title="`${item.label} (${formatScrubberTime(item.marker.timestamp)})`"
                @click.stop="emit('select', item.marker)"
            >
                <span
                    class="h-1.5 w-1.5 shrink-0 rounded-full"
                    :style="{ background: item.color }"
                />
                <span class="truncate">{{ item.label }}</span>
            </button>
        </template>
    </div>
</template>
