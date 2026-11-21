<script setup lang="ts">
import type { SceneListPage, SceneListViewMode, SceneTableColumn } from '~/types/settings';

const settingsStore = useSettingsStore();

const pages: { id: SceneListPage; label: string; hint: string }[] = [
    { id: 'search', label: 'Search', hint: 'Search results' },
    { id: 'actor', label: 'Actor', hint: 'Scenes of an actor' },
    { id: 'studio', label: 'Studio', hint: 'Scenes of a studio' },
    { id: 'playlist', label: 'Playlist', hint: 'Scenes of a playlist (list or table)' },
    { id: 'explorer', label: 'Explorer', hint: 'Scenes of a folder' },
];

const viewOptions: { value: SceneListViewMode; label: string }[] = [
    { value: 'grid', label: 'Grid' },
    { value: 'table', label: 'Table' },
];

// The draft may come from a session saved before this setting existed
const draftView = () => {
    const draft = settingsStore.draft;
    if (!draft) return null;
    if (!draft.scene_list_view) draft.scene_list_view = defaultSceneListView();
    if (!draft.scene_list_view.pages) draft.scene_list_view.pages = {};
    if (!draft.scene_list_view.columns?.length) {
        draft.scene_list_view.columns = [...DEFAULT_SCENE_TABLE_COLUMNS];
    }
    return draft.scene_list_view;
};

const pageMode = (page: SceneListPage): SceneListViewMode =>
    settingsStore.draft?.scene_list_view?.pages?.[page] ?? 'grid';

const setPageMode = (page: SceneListPage, mode: SceneListViewMode) => {
    const view = draftView();
    if (view) view.pages = { ...view.pages, [page]: mode };
};

const visibleColumns = computed(
    () =>
        new Set<SceneTableColumn>(
            settingsStore.draft?.scene_list_view?.columns?.length
                ? settingsStore.draft.scene_list_view.columns
                : DEFAULT_SCENE_TABLE_COLUMNS,
        ),
);

const toggleColumn = (id: SceneTableColumn) => {
    // The title column always shows
    if (id === 'title') return;
    const view = draftView();
    if (!view) return;
    const next = new Set(visibleColumns.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    view.columns = SCENE_TABLE_COLUMNS.map((c) => c.id).filter((c) => next.has(c));
};
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-1 text-sm font-semibold text-white">Scene Lists</h3>
        <p class="text-dim mb-5 text-xs">
            Show scenes as a grid of cards or as a table. The toggle next to each list changes it
            too.
        </p>
        <div class="space-y-4">
            <div
                v-for="page in pages"
                :key="page.id"
                class="flex items-center justify-between gap-4"
            >
                <div>
                    <label class="text-sm font-medium text-white">{{ page.label }}</label>
                    <p class="text-dim mt-0.5 text-xs">{{ page.hint }}</p>
                </div>
                <div class="border-border bg-void/50 flex shrink-0 rounded-lg border p-0.5">
                    <button
                        v-for="opt in viewOptions"
                        :key="opt.value"
                        class="rounded-md px-2.5 py-1 text-[11px] font-medium transition-all"
                        :class="
                            pageMode(page.id) === opt.value
                                ? 'bg-lava/15 text-lava'
                                : 'text-dim hover:text-white'
                        "
                        @click="setPageMode(page.id, opt.value)"
                    >
                        {{ page.id === 'playlist' && opt.value === 'grid' ? 'List' : opt.label }}
                    </button>
                </div>
            </div>
        </div>

        <div class="border-border mt-6 border-t pt-5">
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Table Columns
            </label>
            <p class="text-dim mb-3 text-xs">
                Columns of the scene tables, on every page. Click a sortable header to sort.
            </p>
            <div class="flex flex-wrap gap-1.5">
                <button
                    v-for="col in SCENE_TABLE_COLUMNS"
                    :key="col.id"
                    class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-all"
                    :class="[
                        visibleColumns.has(col.id)
                            ? 'border-lava bg-lava/10 text-lava'
                            : 'border-border text-muted hover:border-border-hover hover:text-white',
                        col.id === 'title' ? 'cursor-default opacity-70' : '',
                    ]"
                    :aria-pressed="visibleColumns.has(col.id)"
                    :title="col.id === 'title' ? 'Always shown' : undefined"
                    @click="toggleColumn(col.id)"
                >
                    {{ col.label }}
                </button>
            </div>
        </div>
    </div>
</template>
