<script setup lang="ts">
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';

// Marker cards of a profile block (versioned thumbnails; a click plays the marker, a range
// loops on the watch page).
const props = defineProps<{ markers: WidgetMarker[]; showcase?: boolean }>();
</script>

<template>
    <div
        class="grid gap-3"
        :class="
            props.showcase
                ? 'grid-cols-[repeat(auto-fill,minmax(9rem,1fr))]'
                : 'grid-cols-[repeat(auto-fill,minmax(8rem,1fr))]'
        "
    >
        <HomepageWidgetsMarkersMarkerCard
            v-for="marker in props.markers"
            :key="marker.id"
            :marker="marker"
        />
    </div>
</template>
