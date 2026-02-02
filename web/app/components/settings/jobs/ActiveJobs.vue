<script setup lang="ts">
import type { JobHistory } from '~/types/jobs';

const props = defineProps<{
    activeJobs: JobHistory[];
}>();

const jobStatusStore = useJobStatusStore();
const { formatDuration, phaseLabel, phaseIcon } = useJobFormatting();

const activeJobsByPhase = computed(() => {
    const phases = ['metadata', 'thumbnail', 'sprites'] as const;
    const result: Record<string, { running: JobHistory[]; queued: JobHistory[] }> = {};
    for (const phase of phases) {
        const phaseJobs = props.activeJobs
            .filter((j) => j.phase === phase)
            .sort((a, b) => new Date(a.started_at).getTime() - new Date(b.started_at).getTime());
        const runningCount = jobStatusStore.byPhase[phase]?.running ?? 0;
        result[phase] = {
            running: phaseJobs.slice(0, runningCount),
            queued: phaseJobs.slice(runningCount),
        };
    }
    return result;
});
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
        <div class="space-y-3">
            <template v-for="phase in ['metadata', 'thumbnail', 'sprites'] as const" :key="phase">
                <div
                    v-if="
                        activeJobsByPhase[phase]?.running.length ||
                        activeJobsByPhase[phase]?.queued.length
                    "
                >
                    <div
                        class="text-dim mb-1.5 flex items-center gap-1.5 text-[10px] font-medium
                            tracking-wider uppercase"
                    >
                        <Icon :name="phaseIcon(phase)" size="11" />
                        {{ phaseLabel(phase) }}
                    </div>
                    <div class="space-y-1.5">
                        <div
                            v-for="job in activeJobsByPhase[phase].running"
                            :key="job.job_id"
                            class="rounded-lg border border-emerald-500/10 bg-emerald-500/5 px-3 py-2"
                        >
                            <div class="flex items-center justify-between">
                                <div class="flex items-center gap-3">
                                    <span
                                        class="inline-block rounded-full border border-emerald-500/30
                                            bg-emerald-500/15 px-2 py-0.5 text-[10px] font-medium
                                            text-emerald-400"
                                    >
                                        running
                                    </span>
                                    <span class="text-[11px] text-white">{{
                                        job.scene_title || `Scene #${job.scene_id}`
                                    }}</span>
                                    <span v-if="job.retry_count > 0" class="text-dim text-[10px]">
                                        (retry {{ job.retry_count }}/{{ job.max_retries }})
                                    </span>
                                </div>
                                <div class="flex items-center gap-2">
                                    <span v-if="job.progress > 0" class="text-[10px] text-emerald-400"
                                        >{{ job.progress }}%</span
                                    >
                                    <span class="text-dim text-[10px]">{{
                                        formatDuration(job.started_at)
                                    }}</span>
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
                        <div
                            v-for="job in activeJobsByPhase[phase].queued"
                            :key="job.job_id"
                            class="flex items-center justify-between rounded-lg border
                                border-amber-500/10 bg-amber-500/5 px-3 py-2"
                        >
                            <div class="flex items-center gap-3">
                                <span
                                    class="inline-block rounded-full border border-amber-500/30
                                        bg-amber-500/15 px-2 py-0.5 text-[10px] font-medium
                                        text-amber-400"
                                >
                                    queued
                                </span>
                                <span class="text-[11px] text-white">{{
                                    job.scene_title || `Scene #${job.scene_id}`
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
</template>
