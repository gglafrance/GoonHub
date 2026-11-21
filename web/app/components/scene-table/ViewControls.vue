<script setup lang="ts">
import type { SceneListPage, SceneListViewMode, SceneTableColumn } from '~/types/settings';

// Grid/table toggle of a scene list page, with the table column picker in table mode
const props = defineProps<{
    page: SceneListPage;
    /** Match the h-10 inputs of the actor/studio pages */
    tall?: boolean;
    /** Label of the non-table view (the playlist page shows a list) */
    gridLabel?: string;
    gridIcon?: string;
    /** Columns this page cannot fill */
    hiddenColumns?: SceneTableColumn[];
}>();

const { mode, isTable } = useSceneListView(props.page);

const options = computed<{ value: SceneListViewMode; label: string; icon: string }[]>(() => [
    {
        value: 'grid',
        label: props.gridLabel ?? 'Grid',
        icon: props.gridIcon ?? 'heroicons:squares-2x2',
    },
    { value: 'table', label: 'Table', icon: 'heroicons:table-cells' },
]);
</script>

<template>
    <div class="flex shrink-0 items-center gap-1.5">
        <div
            class="border-border bg-panel flex items-center rounded-lg border p-0.5"
            :class="tall ? 'h-10' : ''"
            role="group"
            aria-label="Scene list view"
        >
            <button
                v-for="opt in options"
                :key="opt.value"
                type="button"
                class="flex h-full items-center rounded-md px-2 py-1.5 transition-all"
                :class="mode === opt.value ? 'bg-lava/15 text-lava' : 'text-dim hover:text-white'"
                :title="`${opt.label} view`"
                :aria-label="`${opt.label} view`"
                :aria-pressed="mode === opt.value"
                @click="mode = opt.value"
            >
                <Icon :name="opt.icon" size="14" />
            </button>
        </div>
        <SceneTableColumnPicker v-if="isTable" :tall="tall" :hidden-columns="hiddenColumns" />
    </div>
</template>
