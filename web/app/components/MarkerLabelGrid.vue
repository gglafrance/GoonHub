<script setup lang="ts">
import type { MarkerLabelGroup } from '~/types/marker';

defineProps<{
    groups: MarkerLabelGroup[];
    markerThumbnailType?: string;
}>();

const { sceneGridClass, sceneGridStyle } = useGridDensity();
</script>

<template>
    <div class="scene-grid grid justify-center" :class="sceneGridClass" :style="sceneGridStyle">
        <MarkerLabelCard
            v-for="group in groups"
            :key="group.label"
            :group="group"
            :marker-thumbnail-type="markerThumbnailType"
        />
    </div>
</template>
