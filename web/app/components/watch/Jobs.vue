<script setup lang="ts">
import type { JobHistory } from '~/types/jobs';

const route = useRoute();
const { triggerScenePhase, fetchJobs, cancelJob } = useApi();

const sceneId = computed(() => parseInt(route.params.id as string));

const triggeringPhase = ref<string | null>(null);
const cancellingJobId = ref<string | null>(null);
const error = ref('');
const message = ref('');
const loading = ref(true);
const sceneJobs = ref<JobHistory[]>([]);

const loadJobs = async () => {
    loading.value = true;
    try {
        const data = await fetchJobs(1, 100);
        const all: JobHistory[] = [...(data.active_jobs || []), ...(data.data || [])];
        sceneJobs.value = all
            .filter((j) => j.scene_id === sceneId.value)
            .sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime());
    } catch {
        // Non-critical, just show empty
    } finally {
        loading.value = false;
    }
};

const triggerPhase = async (phase: string) => {
    triggeringPhase.value = phase;
    error.value = '';
    message.value = '';
    try {
        await triggerScenePhase(sceneId.value, phase);
        message.value = `${phaseLabel(phase)} job submitted`;
        setTimeout(() => {
            message.value = '';
        }, 4000);
        await loadJobs();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to trigger phase';
    } finally {
        triggeringPhase.value = null;
    }
};

const handleCancel = async (job: JobHistory) => {
    if (cancellingJobId.value) return;
    cancellingJobId.value = job.job_id;
    error.value = '';
    try {
        await cancelJob(job.job_id);
        message.value = `Job cancelled`;
        setTimeout(() => {
            message.value = '';
        }, 4000);
        await loadJobs();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to cancel job';
    } finally {
        cancellingJobId.value = null;
    }
};

const isCancellable = (status: string): boolean => {
    return status === 'running' || status === 'pending';
};

const phaseLabel = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Metadata';
        case 'thumbnail':
            return 'Thumbnail';
        case 'sprites':
            return 'Sprites';
        case 'animated_thumbnails':
            return 'Previews';
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
        case 'animated_thumbnails':
            return 'heroicons:play-circle';
        default:
            return 'heroicons:cog-6-tooth';
    }
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
    });
};

onMounted(() => {
    loadJobs();
});
</script>

<template>
    <div class="space-y-4">
        <!-- Trigger actions -->
        <div class="flex flex-wrap items-center gap-2">
            <span class="text-dim text-[11px]">Run phase:</span>
            <button
                v-for="phase in ['metadata', 'thumbnail', 'sprites', 'animated_thumbnails'] as const"
                :key="phase"
                :disabled="triggeringPhase !== null"
                :class="[
                    `flex cursor-pointer items-center gap-1 rounded-lg border px-3 py-1.5
                    text-[11px] font-medium transition-colors disabled:opacity-50`,
                    triggeringPhase === phase
                        ? 'border-lava/30 bg-lava/15 text-lava'
                        : 'border-white/10 bg-white/5 text-white hover:bg-white/10',
                ]"
                @click="triggerPhase(phase)"
            >
                <Icon :name="phaseIcon(phase)" size="12" />
                {{ triggeringPhase === phase ? 'Submitting...' : phaseLabel(phase) }}
            </button>
        </div>

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

        <!-- Job history for this scene -->
        <div v-if="loading" class="text-dim py-4 text-center text-[11px]">Loading...</div>

        <div v-else-if="sceneJobs.length === 0" class="text-dim py-4 text-center text-[11px]">
            No job history for this scene
        </div>

        <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-xs">
                <thead>
                    <tr
                        class="text-dim border-border border-b text-[10px] tracking-wider uppercase"
                    >
                        <th class="pr-4 pb-2 font-medium">Phase</th>
                        <th class="pr-4 pb-2 font-medium">Status</th>
                        <th class="pr-4 pb-2 font-medium">Duration</th>
                        <th class="pr-4 pb-2 font-medium">Started</th>
                        <th class="pb-2 font-medium"></th>
                    </tr>
                </thead>
                <tbody>
                    <tr
                        v-for="job in sceneJobs"
                        :key="job.job_id"
                        class="border-border/50 border-b last:border-0"
                    >
                        <td class="py-2 pr-4 text-white">
                            <span class="flex items-center gap-1.5">
                                <Icon :name="phaseIcon(job.phase)" size="12" class="text-dim" />
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
                        <td class="text-dim py-2 pr-4 text-[11px]">
                            {{ formatDuration(job.started_at, job.completed_at) }}
                        </td>
                        <td class="text-dim py-2 text-[11px]">
                            {{ formatTime(job.started_at) }}
                        </td>
                        <td class="py-2 text-right">
                            <button
                                v-if="isCancellable(job.status)"
                                :disabled="cancellingJobId !== null"
                                class="text-dim hover:text-lava rounded p-0.5 transition-colors
                                    disabled:opacity-30"
                                title="Cancel job"
                                @click="handleCancel(job)"
                            >
                                <Icon
                                    :name="
                                        cancellingJobId === job.job_id
                                            ? 'heroicons:arrow-path'
                                            : 'heroicons:x-mark'
                                    "
                                    size="12"
                                    :class="{ 'animate-spin': cancellingJobId === job.job_id }"
                                />
                            </button>
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>
    </div>
</template>
