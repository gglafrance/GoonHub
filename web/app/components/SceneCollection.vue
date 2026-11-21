<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { SceneTableColumn } from '~/types/settings';

// The scenes of a list page: cards (selectable or not) or a table, per the page view setting
defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    table?: boolean;
    selectMode?: boolean;
    isSceneSelected: (id: number) => boolean;
    /** Current sort (table headers sort when set) and what an empty sort means */
    sort?: string;
    defaultSort?: string;
    hiddenColumns?: SceneTableColumn[];
}>();

const emit = defineEmits<{
    toggleSelection: [sceneId: number];
    dragSelect: [sceneIds: number[], additive: boolean];
    sort: [value: string];
}>();
</script>

<template>
    <SceneTable
        v-if="table"
        :scenes="scenes"
        :ratings="ratings"
        :sort="sort"
        :default-sort="defaultSort"
        :hidden-columns="hiddenColumns"
        :selectable="selectMode"
        :is-scene-selected="isSceneSelected"
        @sort="emit('sort', $event)"
        @toggle-selection="emit('toggleSelection', $event)"
        @drag-select="(ids, additive) => emit('dragSelect', ids, additive)"
    />
    <SelectableSceneGrid
        v-else-if="selectMode"
        :scenes="scenes"
        :ratings="ratings"
        :likes="likes"
        :jizz-counts="jizzCounts"
        :is-scene-selected="isSceneSelected"
        @toggle-selection="emit('toggleSelection', $event)"
        @drag-select="(ids, additive) => emit('dragSelect', ids, additive)"
    />
    <SceneGrid
        v-else
        :scenes="scenes"
        :ratings="ratings"
        :likes="likes"
        :jizz-counts="jizzCounts"
    />
</template>
