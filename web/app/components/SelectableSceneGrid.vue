<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

// Scene cards with selection: in select mode a click toggles a card and a drag selects the
// cards it covers; outside it, the cards open the scene and a hover checkbox starts a selection.
const props = defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    watched?: Record<string, boolean>;
    isSceneSelected: (id: number) => boolean;
    selectMode: boolean;
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number, range: boolean];
    dragSelect: [sceneIds: number[], additive: boolean];
}>();

const gridRef = ref<HTMLElement | null>(null);
const { sceneGridClass, sceneGridStyle } = useGridDensity();

// Drag selection, only in select mode
const dragContainer = ref<HTMLElement | null>(null);
watchEffect(() => {
    dragContainer.value = props.selectMode ? gridRef.value : null;
});
const { isDragging, selectionRect, dragSelectedIds } = useDragSelect({
    containerRef: dragContainer,
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
            :selectable="selectMode"
            hover-select
            :selected="isSceneSelected(scene.id) || dragSelectedIds.has(scene.id)"
            :class="{ 'pointer-events-none': isDragging }"
            @toggle-selection="(id, range) => emit('toggleSelection', id, range)"
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
