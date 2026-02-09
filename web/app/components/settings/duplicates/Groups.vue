<script setup lang="ts">
import type { DuplicateGroup } from '~/types/duplicates';

const { listGroups, getGroup, resolveGroup, dismissGroup, setWinner, deleteGroup } = useApiDuplicates();

const loading = ref(true);
const error = ref('');
const success = ref('');
const groups = ref<DuplicateGroup[]>([]);
const totalItems = ref(0);
const page = ref(1);
const limit = ref(20);
const statusFilter = ref('');
const expandedGroupId = ref<number | null>(null);
const expandedGroup = ref<DuplicateGroup | null>(null);
const loadingGroupId = ref<number | null>(null);
const actionLoading = ref(false);

const statusFilters = [
    { value: '', label: 'All' },
    { value: 'pending', label: 'Pending' },
    { value: 'resolved', label: 'Resolved' },
    { value: 'dismissed', label: 'Dismissed' },
];

const totalPages = computed(() => Math.ceil(totalItems.value / limit.value) || 1);

function formatDuration(seconds: number): string {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    return `${m}:${String(s).padStart(2, '0')}`;
}

function formatFileSize(bytes: number): string {
    if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB';
    if (bytes >= 1048576) return (bytes / 1048576).toFixed(0) + ' MB';
    return (bytes / 1024).toFixed(0) + ' KB';
}

function formatResolution(w: number, h: number): string {
    if (h >= 2160) return '4K';
    if (h >= 1440) return '1440p';
    if (h >= 1080) return '1080p';
    if (h >= 720) return '720p';
    if (h >= 480) return '480p';
    return `${w}x${h}`;
}

function statusColor(status: string): string {
    switch (status) {
        case 'pending': return 'text-amber-400';
        case 'resolved': return 'text-emerald-400';
        case 'dismissed': return 'text-white/40';
        default: return 'text-dim';
    }
}

async function loadGroups() {
    loading.value = true;
    error.value = '';
    try {
        const result = await listGroups(page.value, limit.value, statusFilter.value);
        groups.value = result.data;
        totalItems.value = result.pagination.total_items;
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to load groups';
    } finally {
        loading.value = false;
    }
}

async function toggleGroup(groupId: number) {
    if (expandedGroupId.value === groupId) {
        expandedGroupId.value = null;
        expandedGroup.value = null;
        return;
    }
    loadingGroupId.value = groupId;
    try {
        expandedGroup.value = await getGroup(groupId);
        expandedGroupId.value = groupId;
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to load group details';
    } finally {
        loadingGroupId.value = null;
    }
}

async function handleResolve(groupId: number) {
    actionLoading.value = true;
    try {
        await resolveGroup(groupId);
        success.value = 'Group resolved';
        setTimeout(() => (success.value = ''), 3000);
        await loadGroups();
        if (expandedGroupId.value === groupId) {
            expandedGroup.value = await getGroup(groupId);
        }
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to resolve group';
    } finally {
        actionLoading.value = false;
    }
}

async function handleDismiss(groupId: number) {
    actionLoading.value = true;
    try {
        await dismissGroup(groupId);
        success.value = 'Group dismissed';
        setTimeout(() => (success.value = ''), 3000);
        await loadGroups();
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to dismiss group';
    } finally {
        actionLoading.value = false;
    }
}

async function handleSetWinner(groupId: number, sceneId: number) {
    actionLoading.value = true;
    try {
        await setWinner(groupId, sceneId);
        success.value = 'Winner set';
        setTimeout(() => (success.value = ''), 3000);
        expandedGroup.value = await getGroup(groupId);
        await loadGroups();
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to set winner';
    } finally {
        actionLoading.value = false;
    }
}

async function handleDelete(groupId: number) {
    if (!confirm('Delete this duplicate group permanently?')) return;
    actionLoading.value = true;
    try {
        await deleteGroup(groupId);
        success.value = 'Group deleted';
        setTimeout(() => (success.value = ''), 3000);
        if (expandedGroupId.value === groupId) {
            expandedGroupId.value = null;
            expandedGroup.value = null;
        }
        await loadGroups();
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to delete group';
    } finally {
        actionLoading.value = false;
    }
}

function changePage(newPage: number) {
    page.value = newPage;
    loadGroups();
}

function changeFilter(value: string) {
    statusFilter.value = value;
    page.value = 1;
    loadGroups();
}

onMounted(loadGroups);
</script>

<template>
    <div class="glass-panel space-y-4 p-5">
        <div>
            <h3 class="text-sm font-semibold text-white">Duplicate Groups</h3>
            <p class="text-dim mt-1 text-[11px]">Manage detected duplicate groups and choose which scenes to keep.</p>
        </div>

        <div v-if="error" class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs">{{ error }}</div>
        <div v-if="success" class="rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs text-emerald-400">{{ success }}</div>

        <!-- Status Filter -->
        <div class="border-border bg-panel flex items-center rounded-lg border p-0.5">
            <button
                v-for="f in statusFilters"
                :key="f.value"
                class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                :class="statusFilter === f.value
                    ? 'bg-lava/15 text-lava'
                    : 'text-dim hover:text-white'"
                @click="changeFilter(f.value)"
            >
                {{ f.label }}
            </button>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="border-lava h-5 w-5 animate-spin rounded-full border-2 border-t-transparent" />
        </div>

        <!-- Empty State -->
        <div v-else-if="groups.length === 0" class="py-8 text-center">
            <Icon name="heroicons:document-duplicate" class="text-dim mx-auto h-8 w-8" />
            <p class="text-dim mt-2 text-xs">No duplicate groups found</p>
        </div>

        <!-- Groups List -->
        <div v-else class="space-y-2">
            <div
                v-for="group in groups"
                :key="group.id"
                class="border-border rounded-lg border"
            >
                <!-- Group Header -->
                <button
                    class="flex w-full items-center justify-between px-4 py-3 text-left transition-colors hover:bg-white/[0.02]"
                    @click="toggleGroup(group.id)"
                >
                    <div class="flex items-center gap-3">
                        <Icon
                            name="heroicons:chevron-right"
                            class="text-dim h-3.5 w-3.5 transition-transform"
                            :class="{ 'rotate-90': expandedGroupId === group.id }"
                        />
                        <span class="text-xs font-medium text-white">Group #{{ group.id }}</span>
                        <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="statusColor(group.status)">
                            {{ group.status }}
                        </span>
                        <span class="text-dim text-[10px]">{{ group.members?.length || 0 }} scenes</span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="text-dim text-[10px]"><NuxtTime :datetime="group.created_at" /></span>
                        <div v-if="loadingGroupId === group.id" class="border-lava h-3 w-3 animate-spin rounded-full border border-t-transparent" />
                    </div>
                </button>

                <!-- Expanded Group Details -->
                <div v-if="expandedGroupId === group.id && expandedGroup" class="border-border border-t px-4 py-3">
                    <!-- Members -->
                    <div class="space-y-2">
                        <div
                            v-for="member in expandedGroup.members"
                            :key="member.id"
                            class="bg-surface flex items-center gap-3 rounded-lg p-3"
                            :class="{ 'ring-1 ring-emerald-500/30': member.is_winner }"
                        >
                            <!-- Thumbnail -->
                            <div class="h-12 w-20 flex-shrink-0 overflow-hidden rounded bg-black/20">
                                <img
                                    v-if="member.scene?.thumbnail_path"
                                    :src="`/thumbnails/${member.scene_id}?size=sm`"
                                    class="h-full w-full object-cover"
                                    loading="lazy"
                                />
                            </div>

                            <!-- Info -->
                            <div class="min-w-0 flex-1">
                                <div class="flex items-center gap-2">
                                    <span class="truncate text-xs font-medium text-white">{{ member.scene?.title || `Scene #${member.scene_id}` }}</span>
                                    <span v-if="member.is_winner" class="rounded bg-emerald-500/15 px-1.5 py-0.5 text-[9px] font-medium text-emerald-400">WINNER</span>
                                </div>
                                <div class="text-dim mt-0.5 flex items-center gap-3 text-[10px]">
                                    <span v-if="member.scene">{{ formatDuration(member.scene.duration) }}</span>
                                    <span v-if="member.scene">{{ formatResolution(member.scene.width, member.scene.height) }}</span>
                                    <span v-if="member.scene?.video_codec">{{ member.scene.video_codec }}</span>
                                    <span v-if="member.scene">{{ formatFileSize(member.scene.file_size) }}</span>
                                    <span class="text-lava font-mono">{{ member.match_percentage.toFixed(1) }}% match</span>
                                </div>
                            </div>

                            <!-- Set Winner -->
                            <button
                                v-if="!member.is_winner && expandedGroup.status === 'pending'"
                                class="border-border hover:border-emerald-500/30 hover:text-emerald-400 text-dim flex-shrink-0 rounded-lg border px-2 py-1 text-[10px] font-medium transition-colors"
                                :disabled="actionLoading"
                                @click.stop="handleSetWinner(group.id, member.scene_id)"
                            >
                                Set Winner
                            </button>
                        </div>
                    </div>

                    <!-- Group Actions -->
                    <div v-if="expandedGroup.status === 'pending'" class="mt-3 flex items-center gap-2">
                        <button
                            class="bg-lava hover:bg-lava/90 rounded-lg px-3 py-1.5 text-[10px] font-medium text-white transition-colors disabled:opacity-50"
                            :disabled="actionLoading"
                            @click="handleResolve(group.id)"
                        >
                            Auto-Resolve
                        </button>
                        <button
                            class="border-border text-dim hover:text-white rounded-lg border px-3 py-1.5 text-[10px] font-medium transition-colors disabled:opacity-50"
                            :disabled="actionLoading"
                            @click="handleDismiss(group.id)"
                        >
                            Dismiss
                        </button>
                        <button
                            class="text-lava/60 hover:text-lava ml-auto text-[10px] font-medium transition-colors disabled:opacity-50"
                            :disabled="actionLoading"
                            @click="handleDelete(group.id)"
                        >
                            Delete
                        </button>
                    </div>
                    <div v-else class="mt-3 flex items-center gap-2">
                        <button
                            class="text-lava/60 hover:text-lava text-[10px] font-medium transition-colors disabled:opacity-50"
                            :disabled="actionLoading"
                            @click="handleDelete(group.id)"
                        >
                            Delete Group
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="flex items-center justify-center gap-1 pt-2">
            <button
                v-for="p in totalPages"
                :key="p"
                class="h-7 w-7 rounded text-xs font-medium transition-colors"
                :class="p === page ? 'bg-lava text-white' : 'text-dim hover:text-white'"
                @click="changePage(p)"
            >
                {{ p }}
            </button>
        </div>
    </div>
</template>
