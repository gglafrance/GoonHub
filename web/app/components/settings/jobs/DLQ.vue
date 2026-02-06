<script setup lang="ts">
import type { DLQEntry, DLQListResponse } from '~/types/jobs';

const { fetchDLQ, retryFromDLQ, abandonDLQ } = useApi();

const loading = ref(true);
const actionLoading = ref<string | null>(null);
const error = ref('');
const message = ref('');

const entries = ref<DLQEntry[]>([]);
const selectedEntryId = ref<string | null>(null);
const selectedEntry = computed(
    () => entries.value.find((e) => e.job_id === selectedEntryId.value) ?? entries.value[0] ?? null,
);
const stats = ref({ pending_review: 0, retrying: 0, abandoned: 0, total: 0 });
const total = ref(0);
const page = ref(1);
const limit = ref(25);
const statusFilter = ref<string>('');

const totalPages = computed(() => Math.ceil(total.value / limit.value));

const loadDLQ = async () => {
    loading.value = true;
    error.value = '';
    try {
        const data: DLQListResponse = await fetchDLQ(page.value, limit.value, statusFilter.value);
        entries.value = data.data || [];
        stats.value = data.stats;
        total.value = data.total;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load DLQ entries';
    } finally {
        loading.value = false;
    }
};

const handleRetry = async (jobId: string) => {
    actionLoading.value = jobId;
    error.value = '';
    message.value = '';
    try {
        await retryFromDLQ(jobId);
        message.value = 'Job resubmitted successfully';
        setTimeout(() => {
            message.value = '';
        }, 3000);
        loadDLQ();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to retry job';
    } finally {
        actionLoading.value = null;
    }
};

const handleAbandon = async (jobId: string) => {
    actionLoading.value = jobId;
    error.value = '';
    message.value = '';
    try {
        await abandonDLQ(jobId);
        message.value = 'Entry marked as abandoned';
        setTimeout(() => {
            message.value = '';
        }, 3000);
        loadDLQ();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to abandon entry';
    } finally {
        actionLoading.value = null;
    }
};

const changeFilter = (status: string) => {
    statusFilter.value = status;
    page.value = 1;
    loadDLQ();
};

const prevPage = () => {
    if (page.value > 1) {
        page.value--;
        loadDLQ();
    }
};

const nextPage = () => {
    if (page.value < totalPages.value) {
        page.value++;
        loadDLQ();
    }
};

const formatTime = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
};

const statusClass = (status: string): string => {
    switch (status) {
        case 'pending_review':
            return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
        case 'retrying':
            return 'bg-blue-500/15 text-blue-400 border-blue-500/30';
        case 'abandoned':
            return 'bg-white/5 text-dim border-white/10';
        default:
            return 'bg-white/5 text-dim border-white/10';
    }
};

const phaseIcon = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'heroicons:document-text';
        case 'thumbnail':
            return 'heroicons:photo';
        case 'sprites':
            return 'heroicons:squares-2x2';
        case 'animated_thumbnails':
            return 'heroicons:play-circle';
        default:
            return 'heroicons:cog-6-tooth';
    }
};

onMounted(() => {
    loadDLQ();
});
</script>

<template>
    <div class="space-y-5">
        <!-- Stats Panel -->
        <div class="glass-panel p-4">
            <div class="mb-3 flex items-center justify-between">
                <h3 class="text-sm font-semibold text-white">Dead Letter Queue</h3>
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    @click="loadDLQ"
                >
                    Refresh
                </button>
            </div>
            <p class="text-dim mb-4 text-[11px]">
                Jobs that have exhausted all retry attempts are moved here for manual review.
            </p>

            <div class="grid grid-cols-4 gap-3">
                <button
                    :class="[
                        'rounded-lg border px-3 py-2.5 text-left transition-colors',
                        statusFilter === ''
                            ? 'border-white/20 bg-white/5'
                            : 'border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="changeFilter('')"
                >
                    <div class="text-dim mb-1 text-[10px] font-medium tracking-wider uppercase">
                        Total
                    </div>
                    <div class="text-lg font-semibold text-white">{{ stats.total }}</div>
                </button>
                <button
                    :class="[
                        'rounded-lg border px-3 py-2.5 text-left transition-colors',
                        statusFilter === 'pending_review'
                            ? 'border-amber-500/30 bg-amber-500/10'
                            : 'border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="changeFilter('pending_review')"
                >
                    <div class="text-dim mb-1 text-[10px] font-medium tracking-wider uppercase">
                        Pending
                    </div>
                    <div class="text-lg font-semibold text-amber-400">
                        {{ stats.pending_review }}
                    </div>
                </button>
                <button
                    :class="[
                        'rounded-lg border px-3 py-2.5 text-left transition-colors',
                        statusFilter === 'retrying'
                            ? 'border-blue-500/30 bg-blue-500/10'
                            : 'border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="changeFilter('retrying')"
                >
                    <div class="text-dim mb-1 text-[10px] font-medium tracking-wider uppercase">
                        Retrying
                    </div>
                    <div class="text-lg font-semibold text-blue-400">{{ stats.retrying }}</div>
                </button>
                <button
                    :class="[
                        'rounded-lg border px-3 py-2.5 text-left transition-colors',
                        statusFilter === 'abandoned'
                            ? 'border-white/20 bg-white/5'
                            : 'border-white/5 bg-white/2 hover:border-white/10',
                    ]"
                    @click="changeFilter('abandoned')"
                >
                    <div class="text-dim mb-1 text-[10px] font-medium tracking-wider uppercase">
                        Abandoned
                    </div>
                    <div class="text-dim text-lg font-semibold">{{ stats.abandoned }}</div>
                </button>
            </div>
        </div>

        <!-- Alerts -->
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div
            v-if="message"
            class="rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs
                text-emerald-400"
        >
            {{ message }}
        </div>

        <!-- DLQ Table -->
        <div class="glass-panel p-5">
            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div v-else-if="entries.length === 0" class="text-dim py-8 text-center text-xs">
                {{ statusFilter ? 'No entries with this status' : 'No failed jobs in the queue' }}
            </div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr
                            class="text-dim border-border border-b text-[11px] tracking-wider
                                uppercase"
                        >
                            <th class="pr-4 pb-2 font-medium">Scene</th>
                            <th class="pr-4 pb-2 font-medium">Phase</th>
                            <th class="pr-4 pb-2 font-medium">Status</th>
                            <th class="pr-4 pb-2 font-medium">Failures</th>
                            <th class="pr-4 pb-2 font-medium">Created</th>
                            <th class="pb-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="entry in entries"
                            :key="entry.job_id"
                            class="border-border/50 cursor-pointer border-b last:border-0
                                hover:bg-white/2"
                            :class="{ 'bg-white/3': selectedEntryId === entry.job_id }"
                            @click="selectedEntryId = entry.job_id"
                        >
                            <td class="max-w-40 truncate py-2.5 pr-4 text-white">
                                {{ entry.scene_title || `Scene #${entry.scene_id}` }}
                            </td>
                            <td class="text-dim py-2.5 pr-4">
                                <span class="flex items-center gap-1.5">
                                    <Icon :name="phaseIcon(entry.phase)" size="11" />
                                    {{ entry.phase }}
                                </span>
                            </td>
                            <td class="py-2.5 pr-4">
                                <span
                                    class="inline-block rounded-full border px-2 py-0.5 text-[10px]
                                        font-medium"
                                    :class="statusClass(entry.status)"
                                >
                                    {{ entry.status.replace('_', ' ') }}
                                </span>
                            </td>
                            <td class="text-dim py-2.5 pr-4">
                                <span class="text-lava font-medium">{{ entry.failure_count }}</span>
                            </td>
                            <td class="text-dim py-2.5 pr-4 text-[11px]">
                                {{ formatTime(entry.created_at) }}
                            </td>
                            <td class="py-2.5">
                                <div
                                    v-if="entry.status === 'pending_review'"
                                    class="flex items-center gap-2"
                                >
                                    <button
                                        :disabled="actionLoading !== null"
                                        class="rounded px-2 py-1 text-[10px] font-medium
                                            text-emerald-400 transition-colors
                                            hover:bg-emerald-500/10 disabled:opacity-50"
                                        @click="handleRetry(entry.job_id)"
                                    >
                                        Retry
                                    </button>
                                    <button
                                        :disabled="actionLoading !== null"
                                        class="text-dim rounded px-2 py-1 text-[10px] font-medium
                                            transition-colors hover:bg-white/5 hover:text-white
                                            disabled:opacity-50"
                                        @click="handleAbandon(entry.job_id)"
                                    >
                                        Abandon
                                    </button>
                                </div>
                                <div
                                    v-else-if="entry.status === 'retrying'"
                                    class="text-dim text-[10px]"
                                >
                                    Processing...
                                </div>
                                <div v-else class="text-dim text-[10px]">-</div>
                            </td>
                        </tr>
                    </tbody>
                </table>

                <!-- Error Details (shown on hover/click) -->
                <div class="border-border mt-4 border-t pt-4">
                    <div class="text-dim mb-2 text-[10px] font-medium tracking-wider uppercase">
                        Error Details
                        <span v-if="selectedEntry" class="text-white/30 normal-case">
                            &mdash;
                            {{ selectedEntry.scene_title || `Scene #${selectedEntry.scene_id}` }}
                        </span>
                    </div>
                    <div
                        v-if="selectedEntry"
                        class="bg-surface rounded-lg border border-white/5 p-3"
                    >
                        <code class="text-lava text-[11px] break-all">
                            {{ selectedEntry.last_error || 'No error details' }}
                        </code>
                    </div>
                    <div v-else class="text-dim py-2 text-[11px]">
                        Click a row to view its error details
                    </div>
                </div>
            </div>

            <!-- Pagination -->
            <div
                v-if="total > limit"
                class="border-border mt-4 flex items-center justify-between border-t pt-3"
            >
                <span class="text-dim text-[11px]">{{ total }} total entries</span>
                <div class="flex items-center gap-3">
                    <button
                        :disabled="page <= 1"
                        class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                            hover:text-white disabled:opacity-30"
                        @click="prevPage"
                    >
                        Previous
                    </button>
                    <span class="text-dim text-[11px]">{{ page }} / {{ totalPages }}</span>
                    <button
                        :disabled="page >= totalPages"
                        class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                            hover:text-white disabled:opacity-30"
                        @click="nextPage"
                    >
                        Next
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
