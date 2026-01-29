<script setup lang="ts">
import type { StudioListItem } from '~/types/studio';

defineProps<{
    studios: StudioListItem[];
}>();
</script>

<template>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        <StudioCard v-for="studio in studios" :key="studio.id" :studio="studio" />
    </div>
</template>
