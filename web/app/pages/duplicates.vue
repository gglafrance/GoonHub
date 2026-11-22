<script setup lang="ts">
import type {
    DuplicateGroup,
    DuplicateSkippedGroup,
    DuplicateStats,
    DuplicationConfig,
} from '~/types/duplicates';

definePageMeta({
    middleware: 'auth',
});

useHead({ title: 'Duplicate Detection' });

useSeoMeta({
    title: 'Duplicate Detection',
    ogTitle: 'Duplicate Detection - GoonHub',
    description: 'Manage detected duplicate scenes',
    ogDescription: 'Manage detected duplicate scenes',
});

const authStore = useAuthStore();
const {
    listGroups,
    getGroup,
    getStats,
    resolveGroup,
    dismissGroup,
    setBest,
    autoSelectBest,
    resolveBulk,
    cancelDeletion,
    getConfig,
} = useApiDuplicates();

// Redirect non-admins
if (authStore.user?.role !== 'admin') {
    navigateTo('/');
}

// State
const groups = ref<DuplicateGroup[]>([]);
const stats = ref<DuplicateStats>({ unresolved: 0, resolved: 0, dismissed: 0, total: 0 });
const loading = ref(true);
// Page, status tab, sort and opened group live in the URL (Back, reload, shared links)
const page = useUrlPagination();
const limit = ref(20);
const total = ref(0);
const statusFilter = useUrlQueryRef('status', '', (raw) =>
    ['unresolved', 'resolved', 'dismissed'].includes(raw) ? raw : undefined,
);
const sortBy = useUrlString('sort', 'newest');
// Kind of group: identical, containment (clip / compilation), partial, unverified
const kindFilter = useUrlQueryRef('kind', '', (raw) =>
    ['identical', 'containment', 'partial', 'unverified'].includes(raw) ? raw : undefined,
);
const expandedGroupParam = useUrlQueryRef('group', 0, (raw) => {
    const n = Number(raw);
    return Number.isInteger(n) && n > 0 ? n : undefined;
});
const expandedGroupId = computed<number | null>({
    get: () => expandedGroupParam.value || null,
    set: (id) => {
        expandedGroupParam.value = id ?? 0;
    },
});
const mergeMetadata = ref(true);

// Bulk selection and actions
const selectedIds = ref<Set<number>>(new Set());
const bulkBusy = ref(false);
const showBulkConfirm = ref(false);
const bulkMessage = ref<string | null>(null);
// Groups the last automatic action skipped (not all their scenes are identical)
const skippedGroups = ref<DuplicateSkippedGroup[]>([]);
const dupConfig = ref<DuplicationConfig | null>(null);

// Player / compare modal
const playerGroup = ref<DuplicateGroup | null>(null);
const playerSceneId = ref<number | null>(null);

const selectableIds = computed(() =>
    groups.value.filter((g) => g.status === 'unresolved').map((g) => g.id),
);

const toggleSelect = (id: number) => {
    const next = new Set(selectedIds.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds.value = next;
};

const selectAllOnPage = () => {
    selectedIds.value = new Set([...selectedIds.value, ...selectableIds.value]);
};

const clearSelection = () => {
    selectedIds.value = new Set();
};

const handleAutoSelect = async (groupIds: number[]) => {
    if (groupIds.length === 0) return;
    bulkBusy.value = true;
    bulkMessage.value = null;
    skippedGroups.value = [];
    try {
        const { selected, skipped } = await autoSelectBest(groupIds);
        bulkMessage.value =
            `Best variant selected in ${Object.keys(selected).length} group(s)` +
            (skipped.length ? `, ${skipped.length} skipped` : '');
        skippedGroups.value = skipped;
        await loadGroups();
    } catch (e) {
        bulkMessage.value = e instanceof Error ? e.message : 'Auto-select failed';
    } finally {
        bulkBusy.value = false;
    }
};

const confirmBulkResolve = async () => {
    showBulkConfirm.value = false;
    const ids = [...selectedIds.value];
    if (ids.length === 0) return;
    bulkBusy.value = true;
    bulkMessage.value = null;
    skippedGroups.value = [];
    try {
        const result = await resolveBulk(ids, true, mergeMetadata.value);
        const markers = describeMarkerMerge(result.markers);
        bulkMessage.value =
            `${result.resolved.length} group(s) resolved` +
            (result.skipped.length ? `, ${result.skipped.length} skipped` : '') +
            (result.failed.length ? `, ${result.failed.length} failed` : '') +
            (markers ? `. Markers: ${markers}` : '');
        skippedGroups.value = result.skipped;
        selectedIds.value = new Set(result.failed.map((f) => f.group_id));
        await Promise.all([loadGroups(), loadStats()]);
    } catch (e) {
        bulkMessage.value = e instanceof Error ? e.message : 'Bulk resolve failed';
    } finally {
        bulkBusy.value = false;
    }
};

const handleCancelDeletion = async (groupId: number, sceneId: number) => {
    try {
        await cancelDeletion(groupId, sceneId);
        await loadGroups();
    } catch (e) {
        console.error('Failed to cancel deletion:', e);
    }
};

const openPlayer = (group: DuplicateGroup, sceneId: number) => {
    playerGroup.value = group;
    playerSceneId.value = sceneId;
};

const closePlayer = () => {
    playerGroup.value = null;
    playerSceneId.value = null;
};

const totalPages = computed(() => Math.ceil(total.value / limit.value) || 1);

const loadGroups = async () => {
    loading.value = true;
    try {
        const data = await listGroups(
            page.value,
            limit.value,
            statusFilter.value,
            sortBy.value,
            kindFilter.value,
        );
        groups.value = data.data;
        total.value = data.pagination.total_items;
    } catch (e) {
        console.error('Failed to load groups:', e);
    } finally {
        loading.value = false;
    }
};

const loadStats = async () => {
    try {
        stats.value = await getStats();
    } catch (e) {
        console.error('Failed to load stats:', e);
    }
};

const handleResolve = async (groupId: number, bestSceneId: number) => {
    try {
        const result = await resolveGroup(groupId, bestSceneId, mergeMetadata.value);
        const markers = describeMarkerMerge(result.markers);
        bulkMessage.value = `Group #${groupId} resolved` + (markers ? `. Markers: ${markers}` : '');
        expandedGroupId.value = null;
        await Promise.all([loadGroups(), loadStats()]);
    } catch (e) {
        console.error('Failed to resolve group:', e);
    }
};

const handleDismiss = async (groupId: number) => {
    try {
        await dismissGroup(groupId);
        expandedGroupId.value = null;
        await Promise.all([loadGroups(), loadStats()]);
    } catch (e) {
        console.error('Failed to dismiss group:', e);
    }
};

const handleSetBest = async (groupId: number, sceneId: number) => {
    try {
        await setBest(groupId, sceneId);
        await loadGroups();
    } catch (e) {
        console.error('Failed to set best:', e);
    }
};

const setStatus = (s: string) => {
    statusFilter.value = s;
    page.value = 1;
    loadGroups();
};

const setKind = (k: string) => {
    kindFilter.value = k;
    page.value = 1;
    loadGroups();
};

// Opens a skipped group to review it, fetching it on top of the list when not on this page
const openSkippedGroup = async (groupId: number) => {
    if (!groups.value.some((g) => g.id === groupId)) {
        try {
            groups.value = [await getGroup(groupId), ...groups.value];
        } catch (e) {
            console.error('Failed to load group:', e);
            return;
        }
    }
    expandedGroupId.value = groupId;
    await nextTick();
    document
        .getElementById(`duplicate-group-${groupId}`)
        ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
};

const setSort = (s: string) => {
    sortBy.value = s;
    page.value = 1;
    loadGroups();
};

const toggleGroup = (id: number) => {
    expandedGroupId.value = expandedGroupId.value === id ? null : id;
};

const goToPage = (p: number) => {
    page.value = p;
    loadGroups();
};

const statusTabs = [
    { value: '', label: 'All' },
    { value: 'unresolved', label: 'Unresolved' },
    { value: 'resolved', label: 'Resolved' },
    { value: 'dismissed', label: 'Dismissed' },
];

// Pagination: compute visible page numbers with ellipsis
const visiblePages = computed(() => {
    const tp = totalPages.value;
    const cp = page.value;
    if (tp <= 7) return Array.from({ length: tp }, (_, i) => i + 1);

    const pages: (number | null)[] = [1];
    const start = Math.max(2, cp - 1);
    const end = Math.min(tp - 1, cp + 1);

    if (start > 2) pages.push(null);
    for (let i = start; i <= end; i++) pages.push(i);
    if (end < tp - 1) pages.push(null);
    pages.push(tp);

    return pages;
});

onMounted(() => {
    // A group opened from a link (e.g. the watch page's Similar tab) may not be on this page
    const linkedGroupId = expandedGroupId.value;
    Promise.all([loadGroups(), loadStats()]).then(() => {
        if (linkedGroupId && expandedGroupId.value === linkedGroupId) {
            openSkippedGroup(linkedGroupId);
        }
    });
    getConfig()
        .then((cfg: DuplicationConfig) => (dupConfig.value = cfg))
        .catch(() => {});
});
</script>

<template>
    <div class="mx-auto max-w-415 px-4 py-6 sm:px-5">
        <!-- Page Header -->
        <div class="mb-6 flex items-center justify-between">
            <div>
                <h1 class="text-lg font-bold text-white">Duplicate Detection</h1>
                <p class="text-dim mt-1 text-xs">Manage detected duplicate scenes</p>
            </div>
            <div class="flex items-center gap-2">
                <DuplicatesMarkerDedup />
                <NuxtLink
                    to="/settings?tab=duplicates"
                    class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                        items-center gap-1.5 rounded-md border px-3 py-1.5 text-[11px] font-medium
                        transition-all"
                >
                    <Icon name="heroicons:adjustments-horizontal" size="14" />
                    Settings
                </NuxtLink>
            </div>
        </div>

        <!-- Stats Bar -->
        <div class="mb-6 grid grid-cols-3 gap-3">
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-lava text-lg font-bold">{{ stats.unresolved }}</div>
                <div class="text-dim text-[11px]">Unresolved</div>
            </div>
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-emerald text-lg font-bold">{{ stats.resolved }}</div>
                <div class="text-dim text-[11px]">Resolved</div>
            </div>
            <div class="border-border rounded-lg border bg-white/2 p-3">
                <div class="text-dim text-lg font-bold">{{ stats.dismissed }}</div>
                <div class="text-dim text-[11px]">Dismissed</div>
            </div>
        </div>

        <!-- Filter Bar -->
        <div class="mb-4 flex items-center justify-between">
            <div class="flex gap-1">
                <button
                    v-for="s in statusTabs"
                    :key="s.value"
                    class="rounded-md px-2.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        statusFilter === s.value
                            ? 'bg-lava/20 text-lava'
                            : 'text-dim hover:bg-white/5 hover:text-white'
                    "
                    @click="setStatus(s.value)"
                >
                    {{ s.label }}
                </button>
            </div>
            <div class="flex items-center gap-2">
                <DuplicatesKindFilter :model-value="kindFilter" @update:model-value="setKind" />
                <select
                    :value="sortBy"
                    class="border-border bg-panel rounded-md border px-2 py-1 text-[11px]
                        text-white"
                    @change="setSort(($event.target as HTMLSelectElement).value)"
                >
                    <option value="newest">Newest</option>
                    <option value="size">Group Size</option>
                </select>
                <button
                    class="text-dim hover:text-lava transition-colors"
                    title="Refresh"
                    @click="
                        loadGroups();
                        loadStats();
                    "
                >
                    <Icon name="heroicons:arrow-path" size="16" />
                </button>
            </div>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-20">
            <div
                class="border-lava/30 border-t-lava h-6 w-6 animate-spin rounded-full border-2"
            ></div>
        </div>

        <!-- Empty State -->
        <div v-else-if="groups.length === 0" class="py-20 text-center">
            <Icon name="heroicons:document-duplicate" size="48" class="text-dim mx-auto mb-3" />
            <p class="text-dim text-sm">No duplicate groups found</p>
        </div>

        <!-- Bulk actions -->
        <DuplicatesBulkBar
            v-if="!loading && groups.length > 0"
            :selected-count="selectedIds.size"
            :selectable-count="selectableIds.length"
            :busy="bulkBusy"
            :deletion-enabled="dupConfig?.delete_resolved_files ?? false"
            :deletion-delay-hours="dupConfig?.deletion_delay_hours ?? 0"
            @select-all="selectAllOnPage"
            @clear="clearSelection"
            @auto-select="handleAutoSelect([...selectedIds])"
            @auto-resolve="showBulkConfirm = true"
        />
        <div
            v-if="bulkMessage"
            class="border-border text-dim mb-3 flex items-center justify-between rounded-md border
                px-3 py-1.5 text-[11px]"
        >
            <span>{{ bulkMessage }}</span>
            <button class="hover:text-white" @click="bulkMessage = null">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <DuplicatesSkippedNotice
            v-if="skippedGroups.length"
            :skipped="skippedGroups"
            @open="openSkippedGroup"
            @close="skippedGroups = []"
        />

        <!-- Group List -->
        <div v-if="!loading && groups.length > 0" class="space-y-3">
            <DuplicatesGroupCard
                v-for="group in groups"
                :id="`duplicate-group-${group.id}`"
                :key="group.id"
                v-model:merge-metadata="mergeMetadata"
                :group="group"
                :expanded="expandedGroupId === group.id"
                :selected="selectedIds.has(group.id)"
                @toggle="toggleGroup(group.id)"
                @toggle-select="toggleSelect(group.id)"
                @set-best="(sceneId: number) => handleSetBest(group.id, sceneId)"
                @auto-select="handleAutoSelect([group.id])"
                @resolve="handleResolve(group.id, group.best_scene_id!)"
                @dismiss="handleDismiss(group.id)"
                @play="(sceneId: number) => openPlayer(group, sceneId)"
                @cancel-deletion="(sceneId: number) => handleCancelDeletion(group.id, sceneId)"
            />
        </div>

        <DuplicatesPlayerModal
            :visible="playerGroup !== null"
            :members="playerGroup?.members ?? []"
            :scene-id="playerSceneId"
            @close="closePlayer"
        />

        <UiConfirmModal
            :visible="showBulkConfirm"
            title="Auto-resolve selected groups"
            confirm-label="Resolve"
            @close="showBulkConfirm = false"
            @confirm="confirmBulkResolve"
        >
            <p class="text-dim text-xs">
                In each of the {{ selectedIds.size }} selected group(s), the best variant is
                selected with the configured criteria and kept; the other scenes go to the trash{{
                    mergeMetadata ? ' after their metadata is merged into the kept scene' : ''
                }}.
            </p>
            <p class="text-dim mt-2 text-xs">
                Groups whose scenes are not all identical (a clip and the compilation containing it,
                partial or unverified matches) are skipped and listed afterwards, to be resolved by
                hand.
            </p>
            <p v-if="dupConfig?.delete_resolved_files" class="text-lava mt-2 text-xs">
                Their files will then be permanently deleted
                {{
                    dupConfig.deletion_delay_hours === 0
                        ? 'at the next cleanup'
                        : `after ${dupConfig.deletion_delay_hours} hours`
                }}, except files on read-only storage paths, which are kept.
            </p>
        </UiConfirmModal>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="mt-6 flex items-center justify-center gap-1">
            <button
                :disabled="page <= 1"
                class="text-dim rounded px-2 py-1 text-[11px] hover:text-white disabled:opacity-30"
                @click="goToPage(page - 1)"
            >
                Prev
            </button>
            <template v-for="(p, idx) in visiblePages" :key="idx">
                <span v-if="p === null" class="text-dim px-1 text-[11px]">...</span>
                <button
                    v-else
                    class="rounded px-2.5 py-1 text-[11px] font-medium transition-all"
                    :class="page === p ? 'bg-lava/20 text-lava' : 'text-dim hover:bg-white/5'"
                    @click="goToPage(p)"
                >
                    {{ p }}
                </button>
            </template>
            <button
                :disabled="page >= totalPages"
                class="text-dim rounded px-2 py-1 text-[11px] hover:text-white disabled:opacity-30"
                @click="goToPage(page + 1)"
            >
                Next
            </button>
        </div>
    </div>
</template>
