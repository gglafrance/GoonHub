<script setup lang="ts">
import type { JobHistory, JobListResponse, QueueStatus } from '~/types/jobs';

const { fetchJobs } = useApi();

const activeSubTab = ref<'history' | 'workers' | 'processing' | 'triggers'>('history');

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
const total = ref(0);
const page = ref(1);
const pageSizes = [10, 25, 50] as const;
const limit = ref(Number(localStorage.getItem('jobs-page-size')) || 10);
const retention = ref('');
const error = ref('');
const autoRefresh = ref(localStorage.getItem('jobs-auto-refresh') === 'true');
const autoRefreshInterval = ref<ReturnType<typeof setInterval> | null>(null);

const totalPages = computed(() => Math.ceil(total.value / limit.value));

const activeJobsByPhase = computed(() => {
    const phases = ['metadata', 'thumbnail', 'sprites'] as const;
    const result: Record<string, { running: JobHistory[]; queued: JobHistory[] }> = {};
    for (const phase of phases) {
        const phaseJobs = activeJobs.value
            .filter((j) => j.phase === phase)
            .sort((a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime());
        const runningCount =
            queueStatus.value[`${phase}_running` as keyof typeof queueStatus.value] || 0;
        result[phase] = {
            running: phaseJobs.slice(0, runningCount),
            queued: phaseJobs.slice(runningCount),
        };
    }
    return result;
});

const loadJobs = async (silent = false) => {
    if (!silent) loading.value = true;
    error.value = '';
    try {
        const data: JobListResponse = await fetchJobs(page.value, limit.value);
        historyJobs.value = data.data || [];
        activeJobs.value = data.active_jobs || [];
        queueStatus.value = data.queue_status;
        poolConfig.value = data.pool_config;
        total.value = data.total;
        retention.value = data.retention;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load jobs';
    } finally {
        loading.value = false;
    }
};

onMounted(() => {
    loadJobs();
});

onUnmounted(() => {
    if (autoRefreshInterval.value) {
        clearInterval(autoRefreshInterval.value);
    }
});

watch(
    autoRefresh,
    (enabled) => {
        localStorage.setItem('jobs-auto-refresh', String(enabled));
        if (autoRefreshInterval.value) {
            clearInterval(autoRefreshInterval.value);
            autoRefreshInterval.value = null;
        }
        if (enabled) {
            autoRefreshInterval.value = setInterval(() => loadJobs(true), 5000);
        }
    },
    { immediate: true },
);

const prevPage = () => {
    if (page.value > 1) {
        page.value--;
        loadJobs();
    }
};

const nextPage = () => {
    if (page.value < totalPages.value) {
        page.value++;
        loadJobs();
    }
};

const changePageSize = (size: number) => {
    limit.value = size;
    page.value = 1;
    localStorage.setItem('jobs-page-size', String(size));
    loadJobs();
};

const formatDuration = (startedAt: string, completedAt?: string): string => {
    const start = new Date(startedAt).getTime();
    const end = completedAt ? new Date(completedAt).getTime() : Date.now();
    const ms = end - start;

    if (ms < 1000) return `${ms}ms`;
    const seconds = Math.floor(ms / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSec = seconds % 60;
    return `${minutes}m ${remainingSec}s`;
};

const formatTime = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    });
};

const statusClass = (status: string): string => {
    switch (status) {
        case 'running':
            return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
        case 'completed':
            return 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30';
        case 'failed':
            return 'bg-lava/15 text-lava border-lava/30';
        default:
            return 'bg-white/5 text-dim border-white/10';
    }
};

const phaseLabel = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Metadata';
        case 'thumbnail':
            return 'Thumbnail';
        case 'sprites':
            return 'Sprites';
        default:
            return phase;
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
        default:
            return 'heroicons:cog-6-tooth';
    }
};
</script>

<template>
    <div class="space-y-5">
        <!-- Sub-tab navigation -->
        <div class="flex items-center gap-1">
            <button
                @click="activeSubTab = 'history'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'history'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                History
            </button>
            <button
                @click="activeSubTab = 'workers'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'workers'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Workers
            </button>
            <button
                @click="activeSubTab = 'processing'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'processing'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Processing
            </button>
            <button
                @click="activeSubTab = 'triggers'"
                :class="[
                    'rounded-full px-3 py-1 text-[11px] font-medium transition-colors',
                    activeSubTab === 'triggers'
                        ? 'bg-white/10 text-white'
                        : 'text-dim hover:text-white',
                ]"
            >
                Triggers
            </button>
        </div>

        <!-- History sub-tab -->
        <template v-if="activeSubTab === 'history'">
            <div
                v-if="error"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ error }}
            </div>

            <!-- Queue Status Panel -->
            <div class="glass-panel p-4">
                <div class="mb-3 flex items-center justify-between">
                    <h3 class="text-sm font-semibold text-white">Queue Status</h3>
                    <div class="flex items-center gap-3">
                        <label class="flex cursor-pointer items-center gap-1.5">
                            <span class="text-dim text-[11px]">Auto</span>
                            <button
                                @click="autoRefresh = !autoRefresh"
                                :class="[
                                    'relative h-4 w-7 rounded-full transition-colors',
                                    autoRefresh ? 'bg-emerald-500' : 'bg-white/10',
                                ]"
                            >
                                <span
                                    :class="[
                                        `absolute top-0.5 left-0.5 h-3 w-3 rounded-full bg-white
                                        transition-transform`,
                                        autoRefresh ? 'translate-x-3' : 'translate-x-0',
                                    ]"
                                ></span>
                            </button>
                        </label>
                        <button
                            @click="() => loadJobs()"
                            class="text-dim text-[11px] transition-colors hover:text-white"
                        >
                            Refresh
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-3 gap-3">
                    <div
                        v-for="phase in ['metadata', 'thumbnail', 'sprites'] as const"
                        :key="phase"
                        class="rounded-lg border border-white/5 bg-white/2 px-3 py-2.5"
                    >
                        <div
                            class="text-dim mb-2 flex items-center gap-1.5 text-[10px] font-medium
                                tracking-wider uppercase"
                        >
                            <Icon :name="phaseIcon(phase)" size="11" />
                            {{ phaseLabel(phase) }}
                        </div>
                        <div class="flex items-baseline gap-3">
                            <div class="flex items-baseline gap-1">
                                <span class="text-dim text-[10px]">W</span>
                                <span class="text-xs font-medium text-white">{{
                                    poolConfig[`${phase}_workers` as keyof typeof poolConfig]
                                }}</span>
                            </div>
                            <div class="flex items-baseline gap-1">
                                <span class="text-[10px] text-emerald-400">R</span>
                                <span
                                    class="text-xs font-medium"
                                    :class="
                                        queueStatus[
                                            `${phase}_running` as keyof typeof queueStatus
                                        ] > 0
                                            ? 'text-emerald-400'
                                            : 'text-dim'
                                    "
                                    >{{
                                        queueStatus[`${phase}_running` as keyof typeof queueStatus]
                                    }}</span
                                >
                            </div>
                            <div class="flex items-baseline gap-1">
                                <span class="text-[10px] text-amber-400">Q</span>
                                <span
                                    class="text-xs font-medium"
                                    :class="
                                        queueStatus[`${phase}_queued` as keyof typeof queueStatus] >
                                        0
                                            ? 'text-amber-400'
                                            : 'text-dim'
                                    "
                                    >{{
                                        queueStatus[`${phase}_queued` as keyof typeof queueStatus]
                                    }}</span
                                >
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Active Jobs (grouped by phase) -->
            <div v-if="activeJobs.length > 0" class="glass-panel p-4">
                <div class="mb-3 flex items-center gap-2">
                    <span class="relative flex h-2 w-2">
                        <span
                            class="absolute inline-flex h-full w-full animate-ping rounded-full
                                bg-emerald-400 opacity-75"
                        ></span>
                        <span
                            class="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"
                        ></span>
                    </span>
                    <h3 class="text-sm font-semibold text-white">
                        Active Jobs
                        <span class="text-dim text-[11px] font-normal"
                            >({{ activeJobs.length }})</span
                        >
                    </h3>
                </div>
                <div class="space-y-3">
                    <template
                        v-for="phase in ['metadata', 'thumbnail', 'sprites'] as const"
                        :key="phase"
                    >
                        <div
                            v-if="
                                activeJobsByPhase[phase]?.running.length ||
                                activeJobsByPhase[phase]?.queued.length
                            "
                        >
                            <div
                                class="text-dim mb-1.5 flex items-center gap-1.5 text-[10px]
                                    font-medium tracking-wider uppercase"
                            >
                                <Icon :name="phaseIcon(phase)" size="11" />
                                {{ phaseLabel(phase) }}
                            </div>
                            <div class="space-y-1.5">
                                <div
                                    v-for="job in activeJobsByPhase[phase].running"
                                    :key="job.job_id"
                                    class="flex items-center justify-between rounded-lg border
                                        border-emerald-500/10 bg-emerald-500/5 px-3 py-2"
                                >
                                    <div class="flex items-center gap-3">
                                        <span
                                            class="inline-block rounded-full border
                                                border-emerald-500/30 bg-emerald-500/15 px-2 py-0.5
                                                text-[10px] font-medium text-emerald-400"
                                        >
                                            running
                                        </span>
                                        <span class="text-[11px] text-white">{{
                                            job.video_title || `Video #${job.video_id}`
                                        }}</span>
                                    </div>
                                    <span class="text-dim text-[10px]">{{
                                        formatDuration(job.started_at)
                                    }}</span>
                                </div>
                                <div
                                    v-for="job in activeJobsByPhase[phase].queued"
                                    :key="job.job_id"
                                    class="flex items-center justify-between rounded-lg border
                                        border-amber-500/10 bg-amber-500/5 px-3 py-2"
                                >
                                    <div class="flex items-center gap-3">
                                        <span
                                            class="inline-block rounded-full border
                                                border-amber-500/30 bg-amber-500/15 px-2 py-0.5
                                                text-[10px] font-medium text-amber-400"
                                        >
                                            queued
                                        </span>
                                        <span class="text-[11px] text-white">{{
                                            job.video_title || `Video #${job.video_id}`
                                        }}</span>
                                    </div>
                                    <span class="text-dim text-[10px]">{{
                                        formatDuration(job.started_at)
                                    }}</span>
                                </div>
                            </div>
                        </div>
                    </template>
                </div>
            </div>

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
                                <th class="pr-4 pb-2 font-medium">Video</th>
                                <th class="pr-4 pb-2 font-medium">Phase</th>
                                <th class="pr-4 pb-2 font-medium">Status</th>
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
                                <td class="max-w-45 truncate py-2 pr-4 text-white">
                                    {{ job.video_title || `Video #${job.video_id}` }}
                                </td>
                                <td class="text-dim py-2 pr-4">
                                    <span class="flex items-center gap-1.5">
                                        <Icon :name="phaseIcon(job.phase)" size="11" />
                                        {{ phaseLabel(job.phase) }}
                                    </span>
                                </td>
                                <td class="py-2 pr-4">
                                    <span
                                        class="inline-block rounded-full border px-2 py-0.5
                                            text-[10px] font-medium"
                                        :class="statusClass(job.status)"
                                        :title="job.error_message || ''"
                                    >
                                        {{ job.status }}
                                    </span>
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
                                limit === size
                                    ? 'bg-white/10 text-white'
                                    : 'text-dim hover:text-white',
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
        </template>

        <!-- Workers sub-tab -->
        <SettingsJobsWorkers v-if="activeSubTab === 'workers'" />

        <!-- Processing sub-tab -->
        <SettingsJobsProcessing v-if="activeSubTab === 'processing'" />

        <!-- Triggers sub-tab -->
        <SettingsJobsTriggers v-if="activeSubTab === 'triggers'" />
    </div>
</template>
