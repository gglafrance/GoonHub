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

const cancellingJobId = ref<string | null>(null);

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
                    class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400
                        opacity-75"
                ></span>
                <span class="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
            </span>
            <h3 class="text-sm font-semibold text-white">
                Active Jobs
                <span class="text-dim text-[11px] font-normal">({{ activeJobs.length }})</span>
            </h3>
        </div>
        <div class="space-y-1.5">
            <div
                v-for="job in activeJobs"
                :key="job.job_id"
                class="rounded-lg border border-emerald-500/10 bg-emerald-500/5 px-3 py-2"
            >
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <Icon :name="phaseIcon(job.phase)" size="11" class="text-dim" />
                        <span class="text-[11px] text-white">{{
                            job.scene_title || `Scene #${job.scene_id}`
                        }}</span>
                        <span v-if="job.retry_count > 0" class="text-dim text-[10px]">
                            (retry {{ job.retry_count }}/{{ job.max_retries }})
                        </span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="text-dim text-[10px]">{{ phaseLabel(job.phase) }}</span>
                        <span v-if="job.progress > 0" class="text-[10px] text-emerald-400"
                            >{{ job.progress }}%</span
                        >
                        <span class="text-dim text-[10px]">{{
                            formatDuration(job.started_at)
                        }}</span>
                        <button
                            :disabled="cancellingJobId !== null"
                            class="text-dim hover:text-lava rounded p-0.5 transition-colors
                                disabled:opacity-30"
                            title="Cancel job"
                            @click.stop="handleCancel(job)"
                        >
                            <Icon
                                :name="cancellingJobId === job.job_id
                                    ? 'heroicons:arrow-path'
                                    : 'heroicons:x-mark'"
                                size="12"
                                :class="{ 'animate-spin': cancellingJobId === job.job_id }"
                            />
                        </button>
                    </div>
                </div>
                <div
                    v-if="job.progress > 0"
                    class="mt-2 h-1 overflow-hidden rounded-full bg-white/5"
                >
                    <div
                        class="h-full rounded-full bg-emerald-500 transition-all duration-300"
                        :style="{ width: `${job.progress}%` }"
                    ></div>
                </div>
            </div>
        </div>
    </div>
</template>
