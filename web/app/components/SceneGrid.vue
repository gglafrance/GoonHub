<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
}>();
</script>

<template>
    <div class="grid justify-center gap-3" style="grid-template-columns: repeat(auto-fill, 320px)">
        <SceneCard v-for="scene in scenes" :key="scene.id" :scene="scene" />
    </div>
</template>
