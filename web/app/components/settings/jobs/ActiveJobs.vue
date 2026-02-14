<script setup lang="ts">
import type { JobHistory } from '~/types/jobs';

const props = defineProps<{
    activeJobs: JobHistory[];
}>();

const emit = defineEmits<{
    reload: [];
}>();

const { formatDuration, phaseLabel, phaseIcon } = useJobFormatting();
const { cancelJob } = useApiJobs();
const jobStatusStore = useJobStatusStore();

const cancellingJobId = ref<string | null>(null);
const expandedPhases = ref<Set<string>>(new Set());

const phaseOrder = ['metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint'];

const groupedJobs = computed(() => {
    const groups: Record<string, JobHistory[]> = {};
    for (const job of props.activeJobs) {
        if (!groups[job.phase]) {
            groups[job.phase] = [];
        }
        groups[job.phase].push(job);
    }
    return groups;
});

const sortedPhases = computed(() => {
    const phases = Object.keys(groupedJobs.value);
    return phases.sort((a, b) => {
        const ai = phaseOrder.indexOf(a);
        const bi = phaseOrder.indexOf(b);
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
    });
});

// Expand all phases by default when jobs change
watch(
    () => props.activeJobs,
    () => {
        for (const phase of Object.keys(groupedJobs.value)) {
            expandedPhases.value.add(phase);
        }
    },
    { immediate: true },
);

const phaseQueuedCount = (phase: string): number => {
    const ps = jobStatusStore.byPhase[phase];
    if (!ps) return 0;
    return ps.queued + ps.pending;
};

const togglePhase = (phase: string) => {
    if (expandedPhases.value.has(phase)) {
        expandedPhases.value.delete(phase);
    } else {
        expandedPhases.value.add(phase);
    }
};

const handleCancel = async (job: JobHistory) => {
    if (cancellingJobId.value) return;
    cancellingJobId.value = job.job_id;
    try {
        await cancelJob(job.job_id);
        emit('reload');
    } catch {
        // Non-critical: job may have already completed
    } finally {
        cancellingJobId.value = null;
    }
};
</script>

<template>
    <div v-if="activeJobs.length > 0" class="glass-panel p-4">
        <div class="mb-3 flex items-center gap-2">
            <span class="relative flex h-2 w-2">
                <span
                    class="absolute inline-flex h-full w-full animate-ping rounded-full
                        bg-emerald-400 opacity-75"
                ></span>
                <span class="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
            </span>
            <h3 class="text-sm font-semibold text-white">
                Active Jobs
                <span class="text-dim text-[11px] font-normal">({{ activeJobs.length }})</span>
            </h3>
        </div>

        <div class="space-y-2">
            <div v-for="phase in sortedPhases" :key="phase">
                <!-- Phase group header -->
                <button
                    class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left
                        transition-colors hover:bg-white/5"
                    @click="togglePhase(phase)"
                >
                    <Icon
                        name="heroicons:chevron-right"
                        size="10"
                        class="text-dim transition-transform duration-150"
                        :class="{ 'rotate-90': expandedPhases.has(phase) }"
                    />
                    <Icon :name="phaseIcon(phase)" size="11" class="text-dim" />
                    <span class="text-[11px] font-medium text-white/80">{{
                        phaseLabel(phase)
                    }}</span>
                    <span
                        class="rounded-full bg-emerald-500/15 px-1.5 py-0.5 text-[10px]
                            font-medium text-emerald-400"
                    >
                        {{ groupedJobs[phase].length }}
                    </span>
                    <span
                        v-if="phaseQueuedCount(phase) > 0"
                        class="text-dim rounded-full bg-white/5 px-1.5 py-0.5 text-[10px]
                            font-medium"
                    >
                        +{{ phaseQueuedCount(phase) }} queued
                    </span>
                </button>

                <!-- Phase group jobs -->
                <div v-if="expandedPhases.has(phase)" class="mt-1 space-y-1 pl-5">
                    <div
                        v-for="job in groupedJobs[phase]"
                        :key="job.job_id"
                        class="rounded-lg border border-emerald-500/10 bg-emerald-500/5 px-3 py-2"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center gap-3">
                                <span class="text-[11px] text-white">{{
                                    job.scene_title || `Scene #${job.scene_id}`
                                }}</span>
                                <span
                                    v-if="job.retry_count > 0"
                                    class="text-dim text-[10px]"
                                >
                                    (retry {{ job.retry_count }}/{{ job.max_retries }})
                                </span>
                            </div>
                            <div class="flex items-center gap-2">
                                <span
                                    v-if="job.progress > 0"
                                    class="text-[10px] text-emerald-400"
                                    >{{ job.progress }}%</span
                                >
                                <span class="text-dim text-[10px]">{{
                                    formatDuration(job.started_at)
                                }}</span>
                                <button
                                    :disabled="cancellingJobId !== null"
                                    class="text-dim hover:text-lava rounded p-0.5
                                        transition-colors disabled:opacity-30"
                                    title="Cancel job"
                                    @click.stop="handleCancel(job)"
                                >
                                    <Icon
                                        :name="
                                            cancellingJobId === job.job_id
                                                ? 'heroicons:arrow-path'
                                                : 'heroicons:x-mark'
                                        "
                                        size="12"
                                        :class="{
                                            'animate-spin': cancellingJobId === job.job_id,
                                        }"
                                    />
                                </button>
                            </div>
                        </div>
                        <div
                            v-if="job.progress > 0"
                            class="mt-2 h-1 overflow-hidden rounded-full bg-white/5"
                        >
                            <div
                                class="h-full rounded-full bg-emerald-500 transition-all
                                    duration-300"
                                :style="{ width: `${job.progress}%` }"
                            ></div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
