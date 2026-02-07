<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
}>();

const explorerStore = useExplorerStore();

const handleToggleSelection = (sceneId: number) => {
    explorerStore.toggleSceneSelection(sceneId);
};
</script>

<template>
    <div
        class="c_grid grid grid-cols-2 justify-center gap-3 md:grid-cols-3 lg:grid-cols-4
            xl:grid-cols-5"
    >
        <SceneCard
            v-for="scene in scenes"
            :key="scene.id"
            :scene="scene"
            :selectable="true"
            :selected="explorerStore.isSceneSelected(scene.id)"
            fluid
            @toggle-selection="handleToggleSelection"
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
