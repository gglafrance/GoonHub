<script setup lang="ts">
import type { JobHistory, JobListResponse, QueueStatus } from '~/types/jobs';

const { fetchJobs } = useApiJobs();
const { formatDuration, formatTime, statusClass, phaseLabel, phaseIcon } = useJobFormatting();

const loading = ref(false);
const historyJobs = ref<JobHistory[]>([]);
const activeJobs = ref<JobHistory[]>([]);
const queueStatus = ref<QueueStatus>({
    metadata_queued: 0,
    thumbnail_queued: 0,
    sprites_queued: 0,
    metadata_running: 0,
    thumbnail_running: 0,
    sprites_running: 0,
});
const poolConfig = ref({ metadata_workers: 0, thumbnail_workers: 0, sprites_workers: 0 });
const retention = ref('');
const error = ref('');

const loadJobs = async (silent = false) => {
    if (!silent) loading.value = true;
    error.value = '';
    try {
        const data: JobListResponse = await fetchJobs(page.value, limit.value);
        historyJobs.value = data.data || [];
        activeJobs.value = data.active_jobs || [];
        queueStatus.value = data.queue_status;
        poolConfig.value = data.pool_config;
        setTotal(data.total);
        retention.value = data.retention;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load jobs';
    } finally {
        loading.value = false;
    }
};

const { pageSizes, page, limit, total, totalPages, prevPage, nextPage, changePageSize, setTotal } =
    useJobPagination({
        onPageChange: () => loadJobs(),
    });

const { autoRefresh, toggle: toggleAutoRefresh } = useJobAutoRefresh({
    onRefresh: () => loadJobs(true),
});

onMounted(() => {
    loadJobs();
});
</script>

<template>
    <div class="space-y-5">
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- Queue Status Panel -->
        <SettingsJobsQueueStatus
            :queue-status="queueStatus"
            :pool-config="poolConfig"
            :auto-refresh="autoRefresh"
            @toggle-auto-refresh="toggleAutoRefresh"
            @refresh="loadJobs()"
        />

        <!-- Active Jobs -->
        <SettingsJobsActiveJobs :active-jobs="activeJobs" :queue-status="queueStatus" />

        <!-- Job History Table -->
        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">History</h3>
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div
                v-else-if="historyJobs.length === 0 && activeJobs.length === 0"
                class="text-dim py-8 text-center text-xs"
            >
                No job history yet
            </div>
            <div v-else-if="historyJobs.length > 0" class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr
                            class="text-dim border-border border-b text-[11px] tracking-wider
                                uppercase"
                        >
                            <th class="pr-4 pb-2 font-medium">Scene</th>
                            <th class="pr-4 pb-2 font-medium">Phase</th>
                            <th class="pr-4 pb-2 font-medium">Status</th>
                            <th class="pr-4 pb-2 font-medium">Retries</th>
                            <th class="pr-4 pb-2 font-medium">Duration</th>
                            <th class="pb-2 font-medium">Started</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="job in historyJobs"
                            :key="job.job_id"
                            class="border-border/50 border-b last:border-0"
                        >
                            <td class="max-w-40 truncate py-2 pr-4 text-white">
                                {{ job.scene_title || `Scene #${job.scene_id}` }}
                            </td>
                            <td class="text-dim py-2 pr-4">
                                <span class="flex items-center gap-1.5">
                                    <Icon :name="phaseIcon(job.phase)" size="11" />
                                    {{ phaseLabel(job.phase) }}
                                </span>
                            </td>
                            <td class="py-2 pr-4">
                                <span
                                    class="inline-block rounded-full border px-2 py-0.5 text-[10px]
                                        font-medium"
                                    :class="statusClass(job.status)"
                                    :title="job.error_message || ''"
                                >
                                    {{ job.status }}
                                </span>
                            </td>
                            <td class="py-2 pr-4">
                                <span
                                    v-if="job.max_retries > 0"
                                    class="text-dim text-[11px]"
                                    :class="{ 'text-lava': job.retry_count >= job.max_retries }"
                                >
                                    {{ job.retry_count }}/{{ job.max_retries }}
                                </span>
                                <span v-else class="text-dim text-[11px]">-</span>
                            </td>
                            <td class="text-dim py-2 pr-4 text-[11px]">
                                {{ formatDuration(job.started_at, job.completed_at) }}
                            </td>
                            <td class="text-dim py-2 text-[11px]">
                                {{ formatTime(job.started_at) }}
                            </td>
                        </tr>
                    </tbody>
                </table>
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
