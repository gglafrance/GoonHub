<script setup lang="ts">
import type {
    CopyAction,
    SceneCopyFilters,
    SceneCopyItem,
    SceneCopyTotals,
    SceneWithCopies,
} from '~/types/duplicates';
import type { StoragePath } from '~/types/storage';

// Duplicates page > Copies: the files the library scan recorded as copies of a scene (same
// content at another path), to make one the main file, separate it, forget it or delete it.
const { listCopies, listCopyIds, forgetCopiesBulk } = useApiDuplicates();
const { fetchStoragePaths } = useApiStorage();
const { formatSize } = useFormatter();

const filters = ref<SceneCopyFilters>(defaultCopyFilters());
const page = ref(1);
const limit = ref(20);
const scenes = ref<SceneWithCopies[]>([]);
const totals = ref<SceneCopyTotals>({ scenes: 0, copies: 0, size: 0 });
const loading = ref(true);
const error = ref<string | null>(null);
const message = ref<string | null>(null);
const storagePaths = ref<StoragePath[]>([]);
const storagePathMap = computed(() => new Map(storagePaths.value.map((sp) => [sp.id, sp])));

// Selection (copy IDs) and every copy seen, for the sizes of a selection spanning pages
const selected = ref<Set<number>>(new Set());
const known = ref<Map<number, SceneCopyItem>>(new Map());
const bulkBusy = ref(false);
const summary = computed(() => selectionSummary(selected.value, known.value));
const pageCopyIds = computed(() => scenes.value.flatMap((s) => s.copies.map((c) => c.id)));

const rangeError = computed(() => sizeRangeError(filters.value));

const load = async () => {
    if (rangeError.value) return;
    loading.value = true;
    error.value = null;
    try {
        const res = await listCopies(buildCopyQuery(filters.value, page.value, limit.value));
        scenes.value = res.data ?? [];
        totals.value = res.totals;
        const next = new Map(known.value);
        for (const s of scenes.value) for (const c of s.copies) next.set(c.id, c);
        known.value = next;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load copies';
    } finally {
        loading.value = false;
    }
};

onMounted(async () => {
    load();
    try {
        const data = await fetchStoragePaths();
        storagePaths.value = data.storage_paths ?? [];
    } catch {
        storagePaths.value = [];
    }
});

watch(
    filters,
    () => {
        page.value = 1;
        selected.value = new Set();
        load();
    },
    { deep: true },
);
watch(page, load);

const toggle = (id: number) => {
    const next = new Set(selected.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selected.value = next;
};
const setSelected = (ids: number[], select: boolean) => {
    const next = new Set(selected.value);
    for (const id of ids) {
        if (select) next.add(id);
        else next.delete(id);
    }
    selected.value = next;
};
const selectMatching = async () => {
    bulkBusy.value = true;
    try {
        const res = await listCopyIds(buildCopyQuery(filters.value));
        selected.value = new Set(res.ids);
        if (res.total > res.ids.length) {
            message.value = `${res.ids.length} of ${res.total} copies selected (bulk limit ${res.limit})`;
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to select the copies';
    } finally {
        bulkBusy.value = false;
    }
};

// Single actions
const actionCopy = ref<SceneCopyItem | null>(null);
const actionScene = ref<SceneWithCopies | null>(null);
const action = ref<'make-main' | 'separate' | 'forget' | null>(null);
const deleteIds = ref<number[]>([]);
const showDelete = ref(false);
const showBulkForget = ref(false);

const onAction = (scene: SceneWithCopies, act: CopyAction, copy: SceneCopyItem) => {
    if (act === 'delete') {
        deleteIds.value = [copy.id];
        showDelete.value = true;
        return;
    }
    actionScene.value = scene;
    actionCopy.value = copy;
    action.value = act;
};
const deleteCopies = computed(() =>
    deleteIds.value.map((id) => known.value.get(id)).filter((c): c is SceneCopyItem => !!c),
);

const afterChange = async (msg: string, doneIds: number[] = []) => {
    message.value = msg;
    if (doneIds.length) setSelected(doneIds, false);
    await load();
};
const onActionDone = (msg: string) => {
    const id = actionCopy.value?.id;
    action.value = null;
    afterChange(msg, id ? [id] : []);
};
const onDeleted = (msg: string, result: { done: number[] }) => {
    showDelete.value = false;
    afterChange(msg, result.done);
};
const confirmBulkForget = async () => {
    showBulkForget.value = false;
    bulkBusy.value = true;
    try {
        const res = await forgetCopiesBulk([...selected.value]);
        await afterChange(describeCopyBulkResult('forget', res, formatSize), res.done);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to forget the copies';
    } finally {
        bulkBusy.value = false;
    }
};
const clearNotice = () => {
    error.value = null;
    message.value = null;
};
const openBulkDelete = () => {
    deleteIds.value = [...selected.value];
    showDelete.value = true;
};
</script>

<template>
    <div>
        <p class="text-dim mb-4 text-[11px]">
            Files the library scan found with the same content as a scene's file at another path.
            They are not separate scenes. Make one the main file, turn it into its own scene, forget
            it, or delete its file to free disk space.
        </p>

        <div class="mb-4 grid grid-cols-3 gap-3">
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-lg font-bold text-white">{{ totals.scenes }}</div>
                <div class="text-dim text-[11px]">Scenes with copies</div>
            </div>
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-lg font-bold text-white">{{ totals.copies }}</div>
                <div class="text-dim text-[11px]">Copies</div>
            </div>
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-lava text-lg font-bold">{{ formatSize(totals.size) }}</div>
                <div class="text-dim text-[11px]">Taken by the copies</div>
            </div>
        </div>

        <DuplicatesCopiesFilters v-model="filters" :storage-paths="storagePaths" />
        <p v-if="rangeError" class="text-lava mb-3 text-[11px]">{{ rangeError }}</p>

        <DuplicatesCopiesBulkBar
            v-if="totals.copies > 0"
            :selected-count="summary.count"
            :selected-size="summary.size"
            :unknown-count="summary.unknown"
            :page-count="pageCopyIds.length"
            :matching-count="totals.copies"
            :busy="bulkBusy"
            @select-page="setSelected(pageCopyIds, true)"
            @select-matching="selectMatching"
            @clear="selected = new Set()"
            @forget="showBulkForget = true"
            @delete="openBulkDelete"
        />

        <div
            v-if="message || error"
            class="border-border mb-3 flex items-center justify-between rounded-md border px-3
                py-1.5 text-[11px]"
            :class="error ? 'text-lava' : 'text-dim'"
        >
            <span>{{ error || message }}</span>
            <button class="hover:text-white" @click="clearNotice">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>

        <div v-if="loading" class="flex items-center justify-center py-20">
            <div
                class="border-lava/30 border-t-lava h-6 w-6 animate-spin rounded-full border-2"
            ></div>
        </div>
        <div v-else-if="scenes.length === 0" class="py-20 text-center">
            <Icon name="heroicons:document-duplicate" size="48" class="text-dim mx-auto mb-3" />
            <p class="text-dim text-sm">No copies found</p>
        </div>
        <div v-else class="space-y-2">
            <DuplicatesCopiesSceneCard
                v-for="scene in scenes"
                :key="scene.scene_id"
                :scene="scene"
                :storage-paths="storagePathMap"
                :selected="selected"
                @toggle="toggle"
                @toggle-all="setSelected"
                @action="(act: CopyAction, copy: SceneCopyItem) => onAction(scene, act, copy)"
            />
        </div>

        <Pagination
            v-if="totals.scenes > limit"
            v-model="page"
            :total="totals.scenes"
            :limit="limit"
            class="mt-6"
        />

        <DuplicatesCopiesActionModal
            :visible="action !== null"
            :action="action"
            :copy="actionCopy"
            :scene="actionScene"
            @close="action = null"
            @done="onActionDone"
        />
        <DuplicatesCopiesDeleteModal
            :visible="showDelete"
            :copies="deleteCopies"
            :ids="deleteIds"
            @close="showDelete = false"
            @deleted="onDeleted"
        />
        <UiConfirmModal
            :visible="showBulkForget"
            title="Forget the selected copies"
            confirm-label="Forget"
            icon="heroicons:eye-slash"
            @close="showBulkForget = false"
            @confirm="confirmBulkForget"
        >
            <p>
                {{ selected.size }} cop{{ selected.size === 1 ? 'y is' : 'ies are' }} removed from
                this list. The files stay on disk, and the next scan records them again as copies.
            </p>
        </UiConfirmModal>
    </div>
</template>
