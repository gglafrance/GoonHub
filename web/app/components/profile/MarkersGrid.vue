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
                ? 'grid-cols-2 md:grid-cols-4'
                : 'grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6'
        "
    >
        <HomepageWidgetsMarkersMarkerCard
            v-for="marker in props.markers"
            :key="marker.id"
            :marker="marker"
        />
    </div>
</template>
