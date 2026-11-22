<script setup lang="ts">
import type { Marker } from '~/types/marker';
import type { MarkerTimeField } from '~/utils/markerTime';

// Time of a marker card (bottom-right of its thumbnail): the start, and the end of a range
// marker, each clickable to type a new time
const props = defineProps<{
    marker: Pick<Marker, 'timestamp' | 'end_timestamp'>;
    // Time being edited on this card, if any
    editing?: MarkerTimeField | null;
}>();

const emit = defineEmits<{
    edit: [field: MarkerTimeField];
}>();

const hasEnd = computed(() => markerHasRange(props.marker));

const partClass = (field: MarkerTimeField) => [
    'rounded px-0.5 transition-colors hover:bg-white/15 hover:text-white focus:outline-none',
    'focus-visible:ring-1 focus-visible:ring-white/40',
    props.editing === field ? 'bg-lava/30 text-white ring-1 ring-lava/60' : '',
];
</script>

<template>
    <div
        class="absolute right-2 bottom-2 z-10 flex items-center rounded bg-black/70 px-1 py-0.5
            font-mono text-[10px] text-white backdrop-blur-sm"
    >
        <button
            type="button"
            :class="partClass('start')"
            title="Edit the start"
            aria-label="Edit the marker start"
            @click.stop="emit('edit', 'start')"
        >
            {{ formatMarkerTime(marker.timestamp) }}
        </button>
        <template v-if="hasEnd">
            <span class="text-white/50">-</span>
            <button
                type="button"
                :class="partClass('end')"
                title="Edit the end"
                aria-label="Edit the marker end"
                @click.stop="emit('edit', 'end')"
            >
                {{ formatMarkerTime(marker.end_timestamp ?? 0) }}
            </button>
        </template>
    </div>
</template>
