<script setup lang="ts">
import type { StudioListItem } from '~/types/studio';

defineProps<{
    studios: StudioListItem[];
}>();

const { entityGridClass } = useGridDensity();
</script>

<template>
    <div class="grid" :class="entityGridClass">
        <StudioCard v-for="studio in studios" :key="studio.id" :studio="studio" />
    </div>
</template>
