<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
}>();
</script>

<template>
    <div class="grid grid-cols-2 gap-2 sm:gap-3 lg:grid-cols-3 xl:grid-cols-4">
        <SceneCard v-for="scene in scenes" :key="scene.id" :scene="scene" fluid />
    </div>
</template>
