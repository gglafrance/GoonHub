<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    isSceneSelected: (id: number) => boolean;
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number];
}>();
</script>

<template>
    <div class="c_grid grid grid-cols-2 justify-center gap-2 sm:gap-3">
        <SceneCard
            v-for="scene in scenes"
            :key="scene.id"
            :scene="scene"
            :rating="ratings?.[scene.id]"
            :liked="likes?.[scene.id]"
            :jizz-count="jizzCounts?.[scene.id]"
            fluid
            selectable
            :selected="isSceneSelected(scene.id)"
            @toggle-selection="emit('toggleSelection', $event)"
        />
    </div>
</template>

<style scoped>
.c_grid {
    @media (width >= 40rem /* 640px */) {
        grid-template-columns: repeat(auto-fill, 320px);
    }
}
</style>
