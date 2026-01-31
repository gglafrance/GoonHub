<script setup lang="ts">
import type { MarkerLabelGroup } from '~/types/marker';

defineProps<{
    groups: MarkerLabelGroup[];
}>();
</script>

<template>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        <MarkerLabelCard v-for="group in groups" :key="group.label" :group="group" />
    </div>
</template>
