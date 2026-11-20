<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import type { MarkerWidgetPayload } from '~/types/homepageMarkerWidgets';

// recent_markers widget: the markers created most recently.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

const items = computed(() => (props.data.widget as MarkerWidgetPayload | undefined)?.items ?? []);
</script>

<template>
    <div
        v-if="items.length === 0"
        class="border-border text-dim flex h-32 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No markers yet
    </div>
    <div
        v-else
        class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
    >
        <HomepageWidgetsMarkersMarkerCard
            v-for="marker in items"
            :key="marker.id"
            :marker="marker"
        />
    </div>
</template>
