<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router';
import type { SceneListItem } from '~/types/scene';
import type { SceneTableColumn } from '~/types/settings';
import type { SceneTableColumnDef } from '~/utils/sceneTable';

// Scenes as a table, with the columns of the user settings. Sortable headers when the page
// passes its sort; selectable rows (click and drag) in select mode.
const props = defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    /** Current sort order of the page; undefined: the headers do not sort */
    sort?: string;
    /** Sort order meant by an empty sort (page default) */
    defaultSort?: string;
    /** Columns this page cannot fill */
    hiddenColumns?: SceneTableColumn[];
    selectable?: boolean;
    isSceneSelected?: (id: number) => boolean;
    /** Show the position of the scene (playlists) */
    showPosition?: boolean;
    /** Link of a row (default: the watch page) */
    rowLink?: (scene: SceneListItem, index: number) => RouteLocationRaw;
}>();

const emit = defineEmits<{
    sort: [value: string];
    // range: shift-click, select from the last toggled scene to this one
    toggleSelection: [sceneId: number, range: boolean];
    dragSelect: [sceneIds: number[], additive: boolean];
}>();

defineSlots<{
    actions?: (props: { scene: SceneListItem; index: number }) => unknown;
}>();

const settingsStore = useSettingsStore();

const columns = computed(() =>
    settingsStore.sceneListView.columns
        .filter((id) => !props.hiddenColumns?.includes(id))
        .map((id) => sceneTableColumnDef(id))
        .filter((c): c is SceneTableColumnDef => !!c),
);

const currentSort = computed(() => props.sort || props.defaultSort || '');

const isSortable = (col: SceneTableColumnDef) =>
    props.sort !== undefined && !!col.sortAsc && !!col.sortDesc;

const sortDir = (col: SceneTableColumnDef): 'asc' | 'desc' | null => {
    if (!isSortable(col)) return null;
    if (currentSort.value === col.sortAsc) return 'asc';
    if (currentSort.value === col.sortDesc) return 'desc';
    return null;
};

const onHeaderClick = (col: SceneTableColumnDef) => {
    if (!isSortable(col)) return;
    const dir = sortDir(col);
    const next = dir ? (dir === 'asc' ? 'desc' : 'asc') : (col.firstDir ?? 'asc');
    emit('sort', (next === 'asc' ? col.sortAsc : col.sortDesc)!);
};

const linkOf = (scene: SceneListItem, index: number): RouteLocationRaw =>
    props.rowLink ? props.rowLink(scene, index) : `/watch/${scene.id}`;

const isSelected = (id: number) => !!props.isSceneSelected?.(id) || dragSelectedIds.value.has(id);

const onRowClick = (event: MouseEvent, scene: SceneListItem) => {
    if (!props.selectable) return;
    event.preventDefault();
    event.stopPropagation();
    emit('toggleSelection', scene.id, event.shiftKey);
};

// Drag selection, only in select mode
const tableRef = ref<HTMLElement | null>(null);
const dragContainer = ref<HTMLElement | null>(null);
watchEffect(() => {
    dragContainer.value = props.selectable ? tableRef.value : null;
});
const { isDragging, selectionRect, dragSelectedIds } = useDragSelect({
    containerRef: dragContainer,
    onDragEnd: (ids, event) => emit('dragSelect', ids, event.metaKey || event.ctrlKey),
});
</script>

<template>
    <div
        ref="tableRef"
        class="border-border bg-surface overflow-x-auto rounded-lg border"
        :class="{ 'select-none': isDragging }"
    >
        <table class="w-full min-w-[640px] table-fixed text-left text-xs">
            <thead>
                <tr class="border-border text-dim border-b text-[10px] tracking-wider uppercase">
                    <th v-if="selectable || showPosition" class="w-9 px-2 py-2"></th>
                    <th
                        v-for="col in columns"
                        :key="col.id"
                        scope="col"
                        class="px-2 py-2 font-medium whitespace-nowrap"
                        :class="[col.width, col.align === 'right' ? 'text-right' : '']"
                        :aria-sort="
                            sortDir(col) === 'asc'
                                ? 'ascending'
                                : sortDir(col) === 'desc'
                                  ? 'descending'
                                  : undefined
                        "
                    >
                        <button
                            v-if="isSortable(col)"
                            type="button"
                            class="inline-flex items-center gap-0.5 uppercase transition-colors
                                hover:text-white"
                            :class="sortDir(col) ? 'text-lava' : ''"
                            :title="`Sort by ${col.label.toLowerCase()}`"
                            @click="onHeaderClick(col)"
                        >
                            {{ col.id === 'thumbnail' ? '' : col.label }}
                            <Icon
                                v-if="sortDir(col)"
                                :name="
                                    sortDir(col) === 'asc'
                                        ? 'heroicons:chevron-up'
                                        : 'heroicons:chevron-down'
                                "
                                size="11"
                            />
                        </button>
                        <span v-else>{{ col.id === 'thumbnail' ? '' : col.label }}</span>
                    </th>
                    <th v-if="$slots.actions" class="w-24 px-2 py-2"></th>
                </tr>
            </thead>
            <tbody>
                <tr
                    v-for="(scene, index) in scenes"
                    :key="scene.id"
                    :data-scene-id="scene.id"
                    class="border-border group text-muted border-b transition-colors last:border-b-0
                        hover:bg-white/[0.03]"
                    :class="[
                        isSelected(scene.id) ? 'bg-lava/5 hover:bg-lava/10' : '',
                        selectable ? 'cursor-pointer' : '',
                        isDragging ? 'pointer-events-none' : '',
                    ]"
                    @click.capture="onRowClick($event, scene)"
                >
                    <td v-if="selectable" class="px-2 py-1.5">
                        <span
                            class="flex h-4 w-4 items-center justify-center rounded border
                                transition-all"
                            :class="
                                isSelected(scene.id)
                                    ? 'border-lava bg-lava'
                                    : 'border-border bg-void group-hover:border-white/40'
                            "
                        >
                            <Icon
                                v-if="isSelected(scene.id)"
                                name="heroicons:check"
                                size="12"
                                class="text-white"
                            />
                        </span>
                    </td>
                    <td
                        v-else-if="showPosition"
                        class="text-dim px-2 py-1.5 text-center font-mono text-[11px]"
                    >
                        {{ index + 1 }}
                    </td>
                    <td
                        v-for="col in columns"
                        :key="col.id"
                        class="px-2 py-1.5"
                        :class="col.align === 'right' ? 'text-right' : ''"
                    >
                        <SceneTableCell
                            :column="col.id"
                            :scene="scene"
                            :rating="ratings?.[scene.id]"
                            :to="linkOf(scene, index)"
                            :selectable="selectable"
                        />
                    </td>
                    <td v-if="$slots.actions" class="px-2 py-1.5">
                        <slot name="actions" :scene="scene" :index="index" />
                    </td>
                </tr>
            </tbody>
        </table>
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
