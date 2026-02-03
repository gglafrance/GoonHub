<script setup lang="ts">
import type { JobHistory, JobListResponse } from '~/types/jobs';
import type { Scene } from '~/types/scene';

const { fetchJobs, retryJob, retryAllFailed, retryBatchJobs, clearFailedJobs } = useApiJobs();
const { fetchScene } = useApiScenes();
const { formatDuration, formatTime, statusClass, phaseLabel, phaseIcon } = useJobFormatting();

const route = useRoute();

const loading = ref(false);
const historyJobs = ref<JobHistory[]>([]);
const activeJobs = ref<JobHistory[]>([]);
const poolConfig = ref({ metadata_workers: 0, thumbnail_workers: 0, sprites_workers: 0 });
const retention = ref('');
const error = ref('');
const message = ref('');
const statusFilter = ref('');
const selectedJobId = ref<string | null>(null);
const retryingJobId = ref<string | null>(null);

// Bulk operations
const selectedJobIds = ref<Set<string>>(new Set());
const bulkAction = ref<'retry-all' | 'retry-batch' | 'clear' | null>(null);
const confirmClear = ref(false);

// Scene path cache
const scenePathCache = ref<Map<number, string | null>>(new Map());
const loadingScenePath = ref(false);

const selectedJob = computed(
    () => historyJobs.value.find((j) => j.job_id === selectedJobId.value) ?? null,
);

const isFailedFilter = computed(() => statusFilter.value === 'failed');

const allOnPageSelected = computed(() => {
    if (historyJobs.value.length === 0) return false;
    return historyJobs.value.every((j) => selectedJobIds.value.has(j.job_id));
});

const statusFilters = [
    { value: '', label: 'All' },
    { value: 'failed', label: 'Failed' },
    { value: 'completed', label: 'Completed' },
    { value: 'cancelled', label: 'Cancelled' },
    { value: 'timed_out', label: 'Timed Out' },
];

const loadJobs = async (silent = false) => {
    if (!silent) loading.value = true;
    error.value = '';
    try {
        const data: JobListResponse = await fetchJobs(
            page.value,
            limit.value,
            statusFilter.value || undefined,
        );
        historyJobs.value = data.data || [];
        activeJobs.value = data.active_jobs || [];
        poolConfig.value = data.pool_config;
        setTotal(data.total);
        retention.value = data.retention;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load jobs';
    } finally {
        loading.value = false;
    }
};

const changeFilter = (status: string) => {
    statusFilter.value = status;
    selectedJobId.value = null;
    selectedJobIds.value.clear();
    confirmClear.value = false;
    page.value = 1;
    loadJobs();
};

const handleRetry = async (jobId: string) => {
    retryingJobId.value = jobId;
    error.value = '';
    message.value = '';
    try {
        await retryJob(jobId);
        message.value = 'Job resubmitted successfully';
        setTimeout(() => {
            message.value = '';
        }, 3000);
        loadJobs(true);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to retry job';
    } finally {
        retryingJobId.value = null;
    }
};

const toggleSelectAll = () => {
    if (allOnPageSelected.value) {
        historyJobs.value.forEach((j) => selectedJobIds.value.delete(j.job_id));
    } else {
        historyJobs.value.forEach((j) => selectedJobIds.value.add(j.job_id));
    }
};

const toggleSelect = (jobId: string) => {
    if (selectedJobIds.value.has(jobId)) {
        selectedJobIds.value.delete(jobId);
    } else {
        selectedJobIds.value.add(jobId);
    }
};

const showMessage = (msg: string) => {
    message.value = msg;
    setTimeout(() => {
        message.value = '';
    }, 3000);
};

const handleRetryAll = async () => {
    bulkAction.value = 'retry-all';
    error.value = '';
    message.value = '';
    try {
        const result = await retryAllFailed();
        showMessage(result.message || `Retried ${result.retried} failed jobs`);
        selectedJobIds.value.clear();
        loadJobs(true);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to retry all failed jobs';
    } finally {
        bulkAction.value = null;
    }
};

const handleRetryBatch = async () => {
    if (selectedJobIds.value.size === 0) return;
    bulkAction.value = 'retry-batch';
    error.value = '';
    message.value = '';
    try {
        const result = await retryBatchJobs([...selectedJobIds.value]);
        showMessage(result.message || `Retried ${result.retried} jobs`);
        selectedJobIds.value.clear();
        loadJobs(true);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to retry selected jobs';
    } finally {
        bulkAction.value = null;
    }
};

const handleClearFailed = async () => {
    if (!confirmClear.value) {
        confirmClear.value = true;
        setTimeout(() => {
            confirmClear.value = false;
        }, 3000);
        return;
    }
    bulkAction.value = 'clear';
    error.value = '';
    message.value = '';
    confirmClear.value = false;
    try {
        const result = await clearFailedJobs();
        showMessage(result.message || `Cleared ${result.deleted} failed jobs`);
        selectedJobIds.value.clear();
        selectedJobId.value = null;
        loadJobs(true);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to clear failed jobs';
    } finally {
        bulkAction.value = null;
    }
};

// Fetch scene path when a job is selected
const fetchScenePath = async (sceneId: number) => {
    if (scenePathCache.value.has(sceneId)) return;
    loadingScenePath.value = true;
    try {
        const scene: Scene = await fetchScene(sceneId);
        scenePathCache.value.set(sceneId, scene.stored_path || null);
    } catch {
        scenePathCache.value.set(sceneId, null);
    } finally {
        loadingScenePath.value = false;
    }
};

watch(selectedJob, (job) => {
    if (job && job.error_message) {
        fetchScenePath(job.scene_id);
    }
});

const selectedJobPath = computed(() => {
    if (!selectedJob.value) return null;
    return scenePathCache.value.get(selectedJob.value.scene_id) ?? null;
});

const { pageSizes, page, limit, total, totalPages, prevPage, nextPage, changePageSize, setTotal } =
    useJobPagination({
        onPageChange: () => loadJobs(),
    });

const { autoRefresh, toggle: toggleAutoRefresh } = useJobAutoRefresh({
    onRefresh: () => loadJobs(true),
});

const jobStatusStore = useJobStatusStore();

onMounted(() => {
    const statusFromUrl = route.query.status as string;
    if (statusFromUrl && statusFilters.some((f) => f.value === statusFromUrl)) {
        statusFilter.value = statusFromUrl;
    }
    loadJobs();
});

watch(
    () => jobStatusStore.lastReconnectedAt,
    (val) => {
        if (val > 0) loadJobs(true);
    },
);
</script>

<template>
    <div class="space-y-5">
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

        <!-- Queue Status Panel (live via SSE) -->
        <SettingsJobsQueueStatus :pool-config="poolConfig" />

        <!-- Active Jobs -->
        <SettingsJobsActiveJobs :active-jobs="activeJobs" @reload="loadJobs(true)" />

        <!-- Job History Table -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <h3 class="text-sm font-semibold text-white">History</h3>
            </div>

            <!-- Status Filter Pills -->
            <div class="mb-4 flex flex-wrap items-center gap-1.5">
                <button
                    v-for="filter in statusFilters"
                    :key="filter.value"
                    @click="changeFilter(filter.value)"
                    :class="[
                        'rounded-full border px-2.5 py-1 text-[11px] font-medium transition-colors',
                        statusFilter === filter.value
                            ? filter.value === 'failed'
                                ? 'border-lava/30 bg-lava/10 text-lava'
                                : filter.value === 'completed'
                                  ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                                  : filter.value === 'cancelled'
                                    ? 'border-amber-500/30 bg-amber-500/10 text-amber-400'
                                    : filter.value === 'timed_out'
                                      ? 'border-orange-500/30 bg-orange-500/10 text-orange-400'
                                      : 'border-white/20 bg-white/5 text-white'
                            : 'border-white/5 text-dim hover:border-white/10 hover:text-white',
                    ]"
                >
                    {{ filter.label }}
                </button>
            </div>

            <!-- Bulk Action Toolbar (only for failed filter) -->
            <div
                v-if="isFailedFilter && historyJobs.length > 0"
                class="mb-4 flex flex-wrap items-center gap-2"
            >
                <button
                    @click="handleRetryAll"
                    :disabled="bulkAction !== null"
                    class="border-lava/20 text-lava hover:bg-lava/10 rounded border px-2.5 py-1
                        text-[11px] font-medium transition-colors disabled:opacity-50"
                >
                    {{ bulkAction === 'retry-all' ? 'Retrying...' : 'Retry All Failed' }}
                </button>
                <button
                    @click="handleRetryBatch"
                    :disabled="selectedJobIds.size === 0 || bulkAction !== null"
                    class="rounded border border-emerald-500/20 px-2.5 py-1 text-[11px] font-medium
                        text-emerald-400 transition-colors hover:bg-emerald-500/10
                        disabled:opacity-50"
                >
                    {{
                        bulkAction === 'retry-batch'
                            ? 'Retrying...'
                            : `Retry Selected (${selectedJobIds.size})`
                    }}
                </button>
                <button
                    @click="handleClearFailed"
                    :disabled="bulkAction !== null"
                    class="rounded border border-white/10 px-2.5 py-1 text-[11px] font-medium
                        transition-colors disabled:opacity-50"
                    :class="
                        confirmClear
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'text-dim hover:border-white/20 hover:text-white'
                    "
                >
                    {{
                        bulkAction === 'clear'
                            ? 'Clearing...'
                            : confirmClear
                              ? 'Confirm?'
                              : 'Clear Failed'
                    }}
                </button>
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div
                v-else-if="historyJobs.length === 0 && activeJobs.length === 0"
                class="text-dim py-8 text-center text-xs"
            >
                {{ statusFilter ? 'No jobs with this status' : 'No job history yet' }}
            </div>
            <div v-else-if="historyJobs.length > 0" class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr
                            class="text-dim border-border border-b text-[11px] tracking-wider
                                uppercase"
                        >
                            <th v-if="isFailedFilter" class="pr-2 pb-2">
                                <input
                                    type="checkbox"
                                    :checked="allOnPageSelected"
                                    @change="toggleSelectAll"
                                    class="accent-lava h-3 w-3 cursor-pointer"
                                />
                            </th>
                            <th class="pr-4 pb-2 font-medium">Scene</th>
                            <th class="pr-4 pb-2 font-medium">Phase</th>
                            <th class="pr-4 pb-2 font-medium">Status</th>
                            <th class="pr-4 pb-2 font-medium">Retries</th>
                            <th class="pr-4 pb-2 font-medium">Duration</th>
                            <th class="pr-4 pb-2 font-medium">Started</th>
                            <th class="pb-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="job in historyJobs"
                            :key="job.job_id"
                            class="border-border/50 cursor-pointer border-b last:border-0
                                hover:bg-white/2"
                            :class="{ 'bg-white/3': selectedJobId === job.job_id }"
                            @click="selectedJobId = job.job_id"
                        >
                            <td v-if="isFailedFilter" class="py-2.5 pr-2">
                                <input
                                    type="checkbox"
                                    :checked="selectedJobIds.has(job.job_id)"
                                    @click.stop="toggleSelect(job.job_id)"
                                    class="accent-lava h-3 w-3 cursor-pointer"
                                />
                            </td>
                            <td class="max-w-40 truncate py-2.5 pr-4 text-white">
                                {{ job.scene_title || `Scene #${job.scene_id}` }}
                            </td>
                            <td class="text-dim py-2.5 pr-4">
                                <span class="flex items-center gap-1.5">
                                    <Icon :name="phaseIcon(job.phase)" size="11" />
                                    {{ phaseLabel(job.phase) }}
                                </span>
                            </td>
                            <td class="py-2.5 pr-4">
                                <span
                                    class="inline-block rounded-full border px-2 py-0.5 text-[10px]
                                        font-medium"
                                    :class="statusClass(job.status)"
                                    :title="job.error_message || ''"
                                >
                                    {{ job.status }}
                                </span>
                            </td>
                            <td class="py-2.5 pr-4">
                                <span
                                    v-if="job.max_retries > 0"
                                    class="text-dim text-[11px]"
                                    :class="{ 'text-lava': job.retry_count >= job.max_retries }"
                                >
                                    {{ job.retry_count }}/{{ job.max_retries }}
                                </span>
                                <span v-else class="text-dim text-[11px]">-</span>
                            </td>
                            <td class="text-dim py-2.5 pr-4 text-[11px]">
                                {{ formatDuration(job.started_at, job.completed_at) }}
                            </td>
                            <td class="text-dim py-2.5 pr-4 text-[11px]">
                                {{ formatTime(job.started_at) }}
                            </td>
                            <td class="py-2.5">
                                <button
                                    v-if="job.status === 'failed'"
                                    @click.stop="handleRetry(job.job_id)"
                                    :disabled="retryingJobId !== null"
                                    class="rounded px-2 py-1 text-[10px] font-medium
                                        text-emerald-400 transition-colors
                                        hover:bg-emerald-500/10 disabled:opacity-50"
                                >
                                    {{ retryingJobId === job.job_id ? 'Retrying...' : 'Retry' }}
                                </button>
                                <span v-else class="text-dim text-[10px]">-</span>
                            </td>
                        </tr>
                    </tbody>
                </table>

                <!-- Error Details Panel -->
                <div
                    v-if="selectedJob && selectedJob.error_message"
                    class="border-border mt-4 border-t pt-4"
                >
                    <div
                        class="text-dim mb-2 flex items-center gap-2 text-[10px] font-medium
                            tracking-wider uppercase"
                    >
                        <span>
                            Error Details
                            <span class="text-white/30 normal-case">
                                &mdash;
                                {{
                                    selectedJob.scene_title || `Scene #${selectedJob.scene_id}`
                                }}
                            </span>
                        </span>
                        <button
                            @click="navigateTo(`/watch/${selectedJob.scene_id}`)"
                            class="text-lava hover:text-lava/80 ml-auto text-[10px] font-medium
                                normal-case tracking-normal transition-colors"
                        >
                            View Scene
                        </button>
                    </div>
                    <div class="bg-surface rounded-lg border border-white/5 p-3">
                        <code class="text-lava break-all text-[11px]">
                            {{ selectedJob.error_message }}
                        </code>
                    </div>
                    <div
                        v-if="loadingScenePath"
                        class="text-dim mt-2 text-[10px]"
                    >
                        Loading file path...
                    </div>
                    <div
                        v-else-if="selectedJobPath"
                        class="mt-2"
                    >
                        <div class="text-dim mb-1 text-[10px] font-medium tracking-wider uppercase">
                            File Path
                        </div>
                        <code
                            class="text-dim block break-all rounded border border-white/5
                                bg-white/2 px-2 py-1.5 text-[11px]"
                        >
                            {{ selectedJobPath }}
                        </code>
                    </div>
                </div>
            </div>

            <!-- Pagination -->
            <div
                v-if="total > 0"
                class="border-border mt-4 flex items-center justify-between border-t pt-3"
            >
                <div class="flex items-center gap-1">
                    <button
                        v-for="size in pageSizes"
                        :key="size"
                        @click="changePageSize(size)"
                        :class="[
                            'rounded px-1.5 py-0.5 text-[11px] transition-colors',
                            limit === size ? 'bg-white/10 text-white' : 'text-dim hover:text-white',
                        ]"
                    >
                        {{ size }}
                    </button>
                </div>
                <div class="flex items-center gap-3">
                    <button
                        @click="prevPage"
                        :disabled="page <= 1"
                        class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                            hover:text-white disabled:opacity-30"
                    >
                        Previous
                    </button>
                    <span class="text-dim text-[11px]">{{ page }} / {{ totalPages }}</span>
                    <button
                        @click="nextPage"
                        :disabled="page >= totalPages"
                        class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                            hover:text-white disabled:opacity-30"
                    >
                        Next
                    </button>
                </div>
            </div>
        </div>

        <!-- Retention Info -->
        <div v-if="retention" class="text-dim text-center text-[11px]">
            Records older than {{ retention }} are automatically cleaned up
        </div>
    </div>
</template>
