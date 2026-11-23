<script setup lang="ts">
// Settings > Trash: paginated list of the trashed videos with search, filters, totals and bulk
// restore / permanent deletion. Page, sort and filters live in the URL (useTrashUrlState).
import type { TrashBulkResult, TrashFacets, TrashedScene, TrashTotals } from '~/types/trash';

const api = useApiAdmin();
const { formatSize } = useFormatter();
const { message, error, clearMessages } = useSettingsMessage();
const { showSelector, maxLimit } = usePageSize();
const { page, limit, sort, order, filters, setFilters, setSort, toggleOrder } = useTrashUrlState();

const pageSizeMax = computed(() => Math.min(maxLimit.value, 100));
const setLimit = (value: number) => {
    limit.value = value;
    page.value = 1;
};

const loading = ref(false);
const scenes = ref<TrashedScene[]>([]);
const totals = ref<{ filtered: TrashTotals; all: TrashTotals }>({
    filtered: { count: 0, size: 0 },
    all: { count: 0, size: 0 },
});
const retentionDays = ref(7);
const facets = ref<TrashFacets | null>(null);
const filtersOpen = ref(activeTrashFilterCount(filters.value) > 0);
const now = ref(Date.now());

const filterCount = computed(() => activeTrashFilterCount(filters.value));
const filterActive = computed(() => filterCount.value > 0 || !!filters.value.q.trim());
const apiFilters = computed(() => trashFilterParams(filters.value, sort.value, order.value));

// Selection spans pages: sizes and read-only state are remembered for the confirmation
const selectedIds = ref<Set<number>>(new Set());
const knownSizes = new Map<number, number>();
const keptOnDisk = new Set<number>();
const selection = computed(() =>
    summarizeTrashSelection(selectedIds.value, knownSizes, keptOnDisk),
);

let loadSeq = 0;
const loadTrash = async () => {
    const seq = ++loadSeq;
    loading.value = true;
    try {
        const data = await api.listTrash(page.value, limit.value, apiFilters.value);
        if (seq !== loadSeq) return;
        scenes.value = data.data;
        totals.value = data.totals;
        retentionDays.value = data.retention_days;
        now.value = Date.now();
        for (const s of data.data) {
            knownSizes.set(s.id, s.size);
            if (s.kept_on_disk) keptOnDisk.add(s.id);
        }
        // Past the last page (after a deletion or a narrower filter)
        const lastPage = Math.max(1, data.pagination.total_pages);
        if (page.value > lastPage) page.value = lastPage;
    } catch (e: unknown) {
        if (seq === loadSeq) error.value = e instanceof Error ? e.message : 'Failed to load trash';
    } finally {
        if (seq === loadSeq) loading.value = false;
    }
};

const loadFacets = async () => {
    try {
        facets.value = await api.getTrashFacets();
    } catch {
        facets.value = null;
    }
};

const reload = () => Promise.all([loadTrash(), loadFacets()]);

watch(
    () => JSON.stringify([page.value, limit.value, apiFilters.value]),
    () => loadTrash(),
);
// A new filter starts a new selection: no hidden video stays selected
watch(
    () => JSON.stringify(filters.value),
    () => (selectedIds.value = new Set()),
);
onMounted(reload);

const toggleSelect = (id: number) => {
    const next = new Set(selectedIds.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds.value = next;
};

const selectPage = () => {
    selectedIds.value = new Set([...selectedIds.value, ...scenes.value.map((s) => s.id)]);
};

const selectAll = async () => {
    clearMessages();
    try {
        const res = await api.listTrashIds(apiFilters.value);
        for (const item of res.items) knownSizes.set(item.id, item.size);
        selectedIds.value = new Set([...selectedIds.value, ...res.ids]);
        if (res.total > res.ids.length) {
            message.value = `Selected the first ${res.ids.length} of ${res.total} results (in the current sort): bulk actions take at most ${res.limit} videos at a time`;
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to select the results';
    }
};

// Bulk actions, sent in chunks the API accepts
const busy = ref(false);
const runBulk = async (
    ids: number[],
    call: (chunk: number[]) => Promise<TrashBulkResult>,
    action: 'restored' | 'deleted',
) => {
    busy.value = true;
    clearMessages();
    const results: TrashBulkResult[] = [];
    try {
        for (const chunk of chunkIds(ids)) results.push(await call(chunk));
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : `Failed: videos not ${action}`;
    }
    const merged = mergeBulkResults(results);
    if (results.length) {
        const text = describeTrashBulk(action, merged, formatSize);
        if (merged.failed.length) error.value = text;
        else message.value = text;
    }
    const done = new Set(merged.succeeded);
    selectedIds.value = new Set([...selectedIds.value].filter((id) => !done.has(id)));
    busy.value = false;
    await reload();
};

const restore = (ids: number[]) => runBulk(ids, api.restoreTrashBulk, 'restored');

// Permanent deletion: always confirmed in the modal (count, size, files deleted from disk)
const deleteIds = ref<number[]>([]);
const deleteTitle = ref<string | undefined>();
const showDelete = ref(false);
const deleteSummary = computed(() =>
    summarizeTrashSelection(deleteIds.value, knownSizes, keptOnDisk),
);

const askDelete = (ids: number[], title?: string) => {
    if (ids.length === 0) return;
    deleteIds.value = ids;
    deleteTitle.value = title;
    showDelete.value = true;
};

const confirmDelete = async () => {
    await runBulk(deleteIds.value, api.deleteTrashBulk, 'deleted');
    showDelete.value = false;
};

// Empty trash (every trashed video, whatever the filter)
const showEmptyModal = ref(false);
const emptyingTrash = ref(false);
const handleEmptyTrash = async () => {
    emptyingTrash.value = true;
    clearMessages();
    try {
        const result = await api.emptyTrash();
        message.value = `${result.deleted} scenes permanently deleted`;
        selectedIds.value = new Set();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to empty trash';
    } finally {
        emptyingTrash.value = false;
        showEmptyModal.value = false;
        await reload();
    }
};
</script>

<template>
    <div class="space-y-3">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div class="glass-panel space-y-3 p-5">
            <h3 class="text-sm font-semibold text-white">Trash</h3>
            <SettingsTrashTotals
                :all="totals.all"
                :filtered="totals.filtered"
                :filter-active="filterActive"
                :retention-days="retentionDays"
                @empty-trash="showEmptyModal = true"
            />
            <SettingsTrashToolbar
                :query="filters.q"
                :sort="sort"
                :order="order"
                :filters-open="filtersOpen"
                :filter-count="filterCount"
                @update:query="(q: string) => setFilters({ ...filters, q })"
                @update:sort="setSort"
                @toggle-order="toggleOrder"
                @toggle-filters="filtersOpen = !filtersOpen"
                @refresh="reload"
            />
            <SettingsTrashFilters
                v-if="filtersOpen"
                :filters="filters"
                :facets="facets"
                @update:filters="setFilters"
            />
            <SettingsTrashBulkBar
                v-if="totals.filtered.count > 0 || selectedIds.size > 0"
                :selected-count="selection.count"
                :selected-size="selection.size"
                :page-count="scenes.length"
                :result-count="totals.filtered.count"
                :busy="busy"
                @select-page="selectPage"
                @select-all="selectAll"
                @clear="selectedIds = new Set()"
                @restore="restore([...selectedIds])"
                @delete="askDelete([...selectedIds])"
            />

            <Pagination
                v-model="page"
                position="top"
                :total="totals.filtered.count"
                :limit="limit"
                :show-page-size-selector="showSelector"
                :max-limit="pageSizeMax"
                @update:limit="setLimit"
            />

            <div v-if="loading && scenes.length === 0" class="text-dim py-8 text-center text-xs">
                Loading...
            </div>
            <div v-else-if="scenes.length === 0" class="py-8 text-center">
                <Icon name="heroicons:trash" size="32" class="text-dim/50 mx-auto mb-2" />
                <p class="text-dim text-xs">
                    {{ totals.all.count ? 'No video matches the filter' : 'Trash is empty' }}
                </p>
            </div>
            <div v-else class="space-y-1.5" :class="{ 'opacity-60': loading }">
                <SettingsTrashRow
                    v-for="scene in scenes"
                    :key="scene.id"
                    :scene="scene"
                    :selected="selectedIds.has(scene.id)"
                    :now="now"
                    :busy="busy"
                    @toggle="toggleSelect(scene.id)"
                    @restore="restore([scene.id])"
                    @delete="askDelete([scene.id], scene.title)"
                />
            </div>

            <Pagination
                v-model="page"
                :total="totals.filtered.count"
                :limit="limit"
                :show-page-size-selector="showSelector"
                :max-limit="pageSizeMax"
                @update:limit="setLimit"
            />
        </div>
    </div>

    <SettingsTrashDeleteModal
        :visible="showDelete"
        :summary="deleteSummary"
        :title="deleteTitle"
        :loading="busy"
        @close="showDelete = false"
        @confirm="confirmDelete"
    />
    <SettingsTrashEmptyModal
        :visible="showEmptyModal"
        :count="totals.all.count"
        :size="totals.all.size"
        :loading="emptyingTrash"
        @close="showEmptyModal = false"
        @confirm="handleEmptyTrash"
    />
</template>
