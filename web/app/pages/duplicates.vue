<script setup lang="ts">
import type { DuplicateGroup, DuplicateStats } from '~/types/duplicates';

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
const { listGroups, getStats, resolveGroup, dismissGroup, setBest } = useApiDuplicates();

// Redirect non-admins
if (authStore.user?.role !== 'admin') {
    navigateTo('/');
}

// State
const groups = ref<DuplicateGroup[]>([]);
const stats = ref<DuplicateStats>({ unresolved: 0, resolved: 0, dismissed: 0, total: 0 });
const loading = ref(true);
const page = ref(1);
const limit = ref(20);
const total = ref(0);
const statusFilter = ref('');
const sortBy = ref('newest');
const expandedGroupId = ref<number | null>(null);
const mergeMetadata = ref(true);

const totalPages = computed(() => Math.ceil(total.value / limit.value) || 1);

const loadGroups = async () => {
    loading.value = true;
    try {
        const data = await listGroups(page.value, limit.value, statusFilter.value, sortBy.value);
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
        await resolveGroup(groupId, bestSceneId, mergeMetadata.value);
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

const formatDuration = (seconds: number): string => {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    return `${m}:${String(s).padStart(2, '0')}`;
};

const formatSize = (bytes: number): string => {
    if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB';
    if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + ' MB';
    return (bytes / 1024).toFixed(1) + ' KB';
};

const formatBitrate = (bps: number): string => {
    if (bps >= 1000000) return (bps / 1000000).toFixed(1) + ' Mbps';
    return (bps / 1000).toFixed(0) + ' Kbps';
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
    Promise.all([loadGroups(), loadStats()]);
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
            <NuxtLink
                to="/settings?tab=duplicates"
                class="border-border text-dim hover:border-lava/30 hover:text-lava flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[11px] font-medium transition-all"
            >
                <Icon name="heroicons:adjustments-horizontal" size="14" />
                Settings
            </NuxtLink>
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
                    :class="statusFilter === s.value ? 'bg-lava/20 text-lava' : 'text-dim hover:bg-white/5 hover:text-white'"
                    @click="setStatus(s.value)"
                >
                    {{ s.label }}
                </button>
            </div>
            <div class="flex items-center gap-2">
                <select
                    :value="sortBy"
                    class="border-border bg-panel rounded-md border px-2 py-1 text-[11px] text-white"
                    @change="setSort(($event.target as HTMLSelectElement).value)"
                >
                    <option value="newest">Newest</option>
                    <option value="size">Group Size</option>
                </select>
                <button
                    class="text-dim hover:text-lava transition-colors"
                    title="Refresh"
                    @click="loadGroups(); loadStats()"
                >
                    <Icon name="heroicons:arrow-path" size="16" />
                </button>
            </div>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-20">
            <div class="border-lava/30 border-t-lava h-6 w-6 animate-spin rounded-full border-2"></div>
        </div>

        <!-- Empty State -->
        <div v-else-if="groups.length === 0" class="py-20 text-center">
            <Icon name="heroicons:document-duplicate" size="48" class="text-dim mx-auto mb-3" />
            <p class="text-dim text-sm">No duplicate groups found</p>
        </div>

        <!-- Group List -->
        <div v-else class="space-y-3">
            <div
                v-for="group in groups"
                :key="group.id"
                class="border-border overflow-hidden rounded-lg border bg-white/2"
            >
                <!-- Group Header (clickable) -->
                <button
                    class="flex w-full items-center justify-between p-4 text-left transition-colors hover:bg-white/2"
                    @click="toggleGroup(group.id)"
                >
                    <div class="flex items-center gap-3">
                        <!-- Member Thumbnails Preview -->
                        <div class="flex -space-x-2">
                            <div
                                v-for="(member, idx) in group.members.slice(0, 3)"
                                :key="member.scene_id"
                                class="border-void h-10 w-16 overflow-hidden rounded border-2 bg-black/50"
                                :style="{ zIndex: 3 - idx }"
                            >
                                <img
                                    v-if="member.thumbnail_path"
                                    :src="`/thumbnails/${member.scene_id}?size=sm`"
                                    class="h-full w-full object-cover"
                                    :alt="member.title"
                                />
                            </div>
                            <div
                                v-if="group.members.length > 3"
                                class="border-void bg-panel text-dim flex h-10 w-10 items-center justify-center rounded border-2 text-[10px] font-medium"
                            >
                                +{{ group.members.length - 3 }}
                            </div>
                        </div>

                        <div>
                            <div class="flex items-center gap-2">
                                <span class="text-sm font-medium text-white">{{ group.scene_count }} scenes</span>
                                <span
                                    class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold uppercase"
                                    :class="{
                                        'bg-lava/20 text-lava': group.status === 'unresolved',
                                        'bg-emerald/20 text-emerald': group.status === 'resolved',
                                        'bg-white/10 text-dim': group.status === 'dismissed',
                                    }"
                                >
                                    {{ group.status }}
                                </span>
                                <span
                                    v-if="group.members[0]"
                                    class="rounded-full bg-white/5 px-1.5 py-0.5 text-[9px] font-medium"
                                    :class="group.members[0].match_type === 'audio' ? 'text-blue-400' : 'text-purple-400'"
                                >
                                    {{ group.members[0].match_type }}
                                </span>
                            </div>
                            <div class="text-dim mt-0.5 text-[11px]">
                                <NuxtTime :datetime="group.created_at" />
                            </div>
                        </div>
                    </div>

                    <Icon
                        name="heroicons:chevron-down"
                        size="16"
                        class="text-dim transition-transform"
                        :class="{ 'rotate-180': expandedGroupId === group.id }"
                    />
                </button>

                <!-- Expanded Detail -->
                <div v-if="expandedGroupId === group.id" class="border-border border-t">
                    <!-- Comparison Table -->
                    <div class="overflow-x-auto">
                        <table class="w-full text-[11px]">
                            <thead>
                                <tr class="border-border border-b bg-white/2">
                                    <th class="text-dim px-4 py-2 text-left font-medium">Scene</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Resolution</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Duration</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Codec</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Bitrate</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Size</th>
                                    <th class="text-dim px-3 py-2 text-left font-medium">Match</th>
                                    <th class="text-dim px-3 py-2 text-center font-medium">Best</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr
                                    v-for="member in group.members"
                                    :key="member.scene_id"
                                    class="border-border border-b transition-colors last:border-b-0"
                                    :class="{
                                        'bg-emerald/5': member.is_best,
                                        'opacity-50': member.is_trashed,
                                        'hover:bg-white/2': !member.is_best && !member.is_trashed,
                                    }"
                                >
                                    <td class="px-4 py-2.5">
                                        <div class="flex items-center gap-2">
                                            <div class="h-8 w-12 shrink-0 overflow-hidden rounded bg-black/50" :class="{ 'grayscale': member.is_trashed }">
                                                <img
                                                    :src="`/thumbnails/${member.scene_id}?size=sm`"
                                                    class="h-full w-full object-cover"
                                                    :alt="member.title"
                                                />
                                            </div>
                                            <div class="flex items-center gap-1.5">
                                                <NuxtLink
                                                    :to="`/watch/${member.scene_id}`"
                                                    class="max-w-48 truncate font-medium transition-colors"
                                                    :class="member.is_trashed ? 'text-dim' : 'text-white hover:text-lava'"
                                                    @click.stop
                                                >
                                                    {{ member.title }}
                                                </NuxtLink>
                                                <span
                                                    v-if="member.is_trashed"
                                                    class="rounded-full bg-white/5 px-1.5 py-0.5 text-[9px] font-medium text-dim"
                                                >
                                                    Trashed
                                                </span>
                                            </div>
                                        </div>
                                    </td>
                                    <td class="px-3 py-2.5 text-white">{{ member.width }}x{{ member.height }}</td>
                                    <td class="px-3 py-2.5 text-white">{{ formatDuration(member.duration) }}</td>
                                    <td class="px-3 py-2.5 text-white">{{ member.video_codec }}</td>
                                    <td class="px-3 py-2.5 text-white">{{ formatBitrate(member.bit_rate) }}</td>
                                    <td class="px-3 py-2.5 text-white">{{ formatSize(member.size) }}</td>
                                    <td class="px-3 py-2.5">
                                        <span class="text-dim">{{ (member.confidence_score * 100).toFixed(0) }}%</span>
                                    </td>
                                    <td class="px-3 py-2.5 text-center">
                                        <button
                                            v-if="group.status === 'unresolved'"
                                            class="inline-flex h-5 w-5 items-center justify-center rounded-full border transition-all"
                                            :class="member.is_best ? 'border-emerald bg-emerald/20 text-emerald' : 'border-white/20 text-dim hover:border-emerald/50'"
                                            @click.stop="handleSetBest(group.id, member.scene_id)"
                                        >
                                            <Icon v-if="member.is_best" name="heroicons:check" size="12" />
                                        </button>
                                        <Icon v-else-if="member.is_best" name="heroicons:check-circle-solid" size="16" class="text-emerald" />
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>

                    <!-- Actions -->
                    <div v-if="group.status === 'unresolved'" class="border-border flex items-center justify-between border-t p-4">
                        <label class="text-dim flex items-center gap-2 text-[11px]">
                            <input v-model="mergeMetadata" type="checkbox" class="accent-lava rounded" />
                            Merge metadata (tags, actors)
                        </label>
                        <div class="flex gap-2">
                            <button
                                class="border-border text-dim rounded-md border px-3 py-1.5 text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white"
                                @click="handleDismiss(group.id)"
                            >
                                Dismiss
                            </button>
                            <button
                                class="bg-lava hover:bg-lava/80 rounded-md px-3 py-1.5 text-[11px] font-medium text-white transition-all disabled:opacity-50"
                                :disabled="!group.best_scene_id"
                                @click="handleResolve(group.id, group.best_scene_id!)"
                            >
                                Resolve
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

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
