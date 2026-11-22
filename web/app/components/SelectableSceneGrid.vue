<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    watched?: Record<string, boolean>;
    isSceneSelected: (id: number) => boolean;
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number];
    dragSelect: [sceneIds: number[], additive: boolean];
}>();

const gridRef = ref<HTMLElement | null>(null);
const { sceneGridClass, sceneGridStyle } = useGridDensity();

const { isDragging, selectionRect, dragSelectedIds } = useDragSelect({
    containerRef: gridRef,
    onDragEnd: (ids, event) => {
        emit('dragSelect', ids, event.metaKey || event.ctrlKey);
    },
});
</script>

<template>
    <div
        ref="gridRef"
        class="scene-grid grid justify-center"
        :class="[sceneGridClass, { 'select-none': isDragging }]"
        :style="sceneGridStyle"
    >
        <SceneCard
            v-for="scene in scenes"
            :key="scene.id"
            :data-scene-id="scene.id"
            :scene="scene"
            :rating="ratings?.[scene.id]"
            :liked="likes?.[scene.id]"
            :jizz-count="jizzCounts?.[scene.id]"
            :completed="watched?.[scene.id]"
            fluid
            selectable
            :selected="isSceneSelected(scene.id) || dragSelectedIds.has(scene.id)"
            :class="{ 'pointer-events-none': isDragging }"
            @toggle-selection="emit('toggleSelection', $event)"
        />
    </div>

    <Teleport to="body">
        <div
            v-if="selectionRect"
            class="pointer-events-none fixed z-50 rounded border border-[#FF4D4D]/40
                bg-[#FF4D4D]/10"
            :style="{
                left: selectionRect.left + 'px',
                top: selectionRect.top + 'px',
                width: selectionRect.width + 'px',
                height: selectionRect.height + 'px',
            }"
        />
    </Teleport>
</template>
