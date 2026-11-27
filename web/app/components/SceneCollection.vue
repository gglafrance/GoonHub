<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type { SceneTableColumn } from '~/types/settings';

// The scenes of a list page: selectable cards or a table, per the page view setting. The cards
// stay mounted when select mode toggles (their hover checkbox turns it on).
defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    watched?: Record<string, boolean>;
    table?: boolean;
    selectMode?: boolean;
    isSceneSelected: (id: number) => boolean;
    /** Current sort (table headers sort when set) and what an empty sort means */
    sort?: string;
    defaultSort?: string;
    hiddenColumns?: SceneTableColumn[];
}>();

const emit = defineEmits<{
    // range: shift-click, select from the last toggled scene to this one
    toggleSelection: [sceneId: number, range: boolean];
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
        @toggle-selection="(id, range) => emit('toggleSelection', id, range)"
        @drag-select="(ids, additive) => emit('dragSelect', ids, additive)"
    />
    <SelectableSceneGrid
        v-else
        :scenes="scenes"
        :ratings="ratings"
        :likes="likes"
        :jizz-counts="jizzCounts"
        :watched="watched"
        :is-scene-selected="isSceneSelected"
        :select-mode="!!selectMode"
        @toggle-selection="(id, range) => emit('toggleSelection', id, range)"
        @drag-select="(ids, additive) => emit('dragSelect', ids, additive)"
    />
</template>
