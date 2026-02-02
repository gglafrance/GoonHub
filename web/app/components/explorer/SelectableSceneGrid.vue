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
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
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
