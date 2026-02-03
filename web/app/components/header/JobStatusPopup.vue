<script setup lang="ts">
import type { JobHistory } from '~/types/jobs';

const props = defineProps<{
    visible: boolean;
    anchorEl: HTMLElement | null;
}>();

const emit = defineEmits<{
    close: [];
}>();

const jobStatusStore = useJobStatusStore();
const { fetchRecentFailedJobs, retryJob } = useApiJobs();
const popupRef = ref<HTMLDivElement | null>(null);
const position = ref({ top: 0, left: 0 });

const phases = ['metadata', 'thumbnail', 'sprites'] as const;

const phaseLabels: Record<string, string> = {
    metadata: 'Metadata',
    thumbnail: 'Thumbnails',
    sprites: 'Sprites',
};

const phaseIcons: Record<string, string> = {
    metadata: 'heroicons:document-text',
    thumbnail: 'heroicons:photo',
    sprites: 'heroicons:squares-2x2',
};

const recentFailures = ref<JobHistory[]>([]);
const failuresLoading = ref(false);
const retryingJobId = ref<string | null>(null);

async function loadRecentFailures() {
    failuresLoading.value = true;
    try {
        const data = await fetchRecentFailedJobs(3);
        recentFailures.value = data.data || [];
    } catch {
        recentFailures.value = [];
    } finally {
        failuresLoading.value = false;
    }
}

async function handleRetryJob(jobId: string) {
    retryingJobId.value = jobId;
    try {
        await retryJob(jobId);
        recentFailures.value = recentFailures.value.filter((j) => j.job_id !== jobId);
    } catch {
        // Silently fail - the button will reset
    } finally {
        retryingJobId.value = null;
    }
}

function navigateToFailedJobs() {
    emit('close');
    navigateTo('/settings?tab=jobs&subtab=history&status=failed');
}

function updatePosition() {
    if (!props.anchorEl) return;
    const rect = props.anchorEl.getBoundingClientRect();
    const popupWidth = 320; // w-80 = 320px
    const padding = 8;
    const viewportWidth = window.innerWidth;

    // Center under anchor, but clamp to viewport bounds
    let left = rect.left + rect.width / 2 - popupWidth / 2;
    left = Math.max(padding, Math.min(left, viewportWidth - popupWidth - padding));

    position.value = {
        top: rect.bottom + 8,
        left,
    };
}

function onClickOutside(e: MouseEvent) {
    if (
        popupRef.value &&
        !popupRef.value.contains(e.target as Node) &&
        props.anchorEl &&
        !props.anchorEl.contains(e.target as Node)
    ) {
        emit('close');
    }
}

function formatElapsed(startedAt: string): string {
    const start = new Date(startedAt);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - start.getTime()) / 1000);

    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
}

function navigateToJobs() {
    emit('close');
    navigateTo('/settings?tab=jobs');
}

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            updatePosition();
            window.addEventListener('resize', updatePosition);
            document.addEventListener('mousedown', onClickOutside);
            if (jobStatusStore.hasFailed) {
                loadRecentFailures();
            }
        } else {
            window.removeEventListener('resize', updatePosition);
            document.removeEventListener('mousedown', onClickOutside);
        }
    },
);

onBeforeUnmount(() => {
    window.removeEventListener('resize', updatePosition);
    document.removeEventListener('mousedown', onClickOutside);
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            ref="popupRef"
            class="border-border bg-panel/95 fixed z-50 w-80 rounded-lg border shadow-2xl
                backdrop-blur-md"
            :style="{ top: `${position.top}px`, left: `${position.left}px` }"
        >
            <!-- Header -->
            <div class="border-border flex items-center justify-between border-b px-4 py-3">
                <div class="flex items-center gap-2">
                    <Icon name="heroicons:bolt" size="16" class="text-lava" />
                    <span class="text-xs font-semibold text-white">Job Status</span>
                </div>
                <div
                    v-if="!jobStatusStore.isConnected"
                    class="flex items-center gap-1.5 text-amber-500"
                >
                    <Icon name="heroicons:exclamation-triangle" size="12" />
                    <span class="text-[10px]">Disconnected</span>
                </div>
            </div>

            <!-- Phase Breakdown -->
            <div class="border-border border-b px-4 py-3">
                <div class="mb-2 flex items-center justify-between">
                    <span class="text-[10px] font-medium tracking-wider text-white/40 uppercase">
                        By Phase
                    </span>
                    <span class="text-[9px] text-white/25">
                        run / wait / fail
                    </span>
                </div>
                <div class="space-y-2">
                    <div
                        v-for="phase in phases"
                        :key="phase"
                        class="flex items-center justify-between"
                    >
                        <div class="flex items-center gap-2">
                            <Icon :name="phaseIcons[phase] ?? ''" size="12" class="text-dim" />
                            <span class="text-xs text-white/70">{{
                                phaseLabels[phase] ?? phase
                            }}</span>
                        </div>
                        <div class="flex items-center gap-3">
                            <div class="flex items-center gap-1">
                                <span
                                    class="text-xs font-medium"
                                    :class="
                                        (jobStatusStore.byPhase[phase]?.running ?? 0) > 0
                                            ? 'text-emerald'
                                            : 'text-white/40'
                                    "
                                >
                                    {{ jobStatusStore.byPhase[phase]?.running ?? 0 }}
                                </span>
                                <span class="text-[10px] text-white/30">run</span>
                            </div>
                            <div class="flex items-center gap-1">
                                <span
                                    class="text-xs font-medium"
                                    :class="
                                        (jobStatusStore.byPhase[phase]?.queued ?? 0) +
                                            (jobStatusStore.byPhase[phase]?.pending ?? 0) >
                                        0
                                            ? 'text-amber-400'
                                            : 'text-white/40'
                                    "
                                >
                                    {{
                                        (jobStatusStore.byPhase[phase]?.queued ?? 0) +
                                        (jobStatusStore.byPhase[phase]?.pending ?? 0)
                                    }}
                                </span>
                                <span class="text-[10px] text-white/30">wait</span>
                            </div>
                            <button
                                class="flex items-center gap-1 transition-colors"
                                :class="
                                    (jobStatusStore.byPhase[phase]?.failed ?? 0) > 0
                                        ? 'cursor-pointer hover:opacity-80'
                                        : 'cursor-default'
                                "
                                @click="
                                    (jobStatusStore.byPhase[phase]?.failed ?? 0) > 0 &&
                                        navigateToFailedJobs()
                                "
                            >
                                <span
                                    class="text-xs font-medium"
                                    :class="
                                        (jobStatusStore.byPhase[phase]?.failed ?? 0) > 0
                                            ? 'text-red-400'
                                            : 'text-white/40'
                                    "
                                >
                                    {{ jobStatusStore.byPhase[phase]?.failed ?? 0 }}
                                </span>
                                <span class="text-[10px] text-white/30">fail</span>
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Recent Failures -->
            <div
                v-if="jobStatusStore.hasFailed"
                class="border-border border-b px-4 py-3"
            >
                <div class="mb-2 flex items-center justify-between">
                    <span
                        class="text-[10px] font-medium tracking-wider text-red-400/70 uppercase"
                    >
                        Recent Failures
                    </span>
                    <button
                        class="text-[9px] text-red-400/50 transition-colors hover:text-red-400"
                        @click="navigateToFailedJobs"
                    >
                        View all
                    </button>
                </div>

                <div v-if="failuresLoading" class="py-2">
                    <span class="text-dim text-xs">Loading...</span>
                </div>

                <div
                    v-else-if="recentFailures.length === 0"
                    class="py-2"
                >
                    <span class="text-xs text-white/40">No recent failures</span>
                </div>

                <div v-else class="space-y-2">
                    <div
                        v-for="job in recentFailures"
                        :key="job.job_id"
                        class="bg-void/50 rounded-md p-2"
                    >
                        <div class="flex items-center gap-2">
                            <div class="min-w-0 flex-1">
                                <div class="flex items-center gap-1.5">
                                    <Icon
                                        :name="phaseIcons[job.phase] ?? ''"
                                        size="10"
                                        class="shrink-0 text-red-400"
                                    />
                                    <span
                                        class="truncate text-xs text-white"
                                        :title="job.scene_title || `Scene #${job.scene_id}`"
                                    >
                                        {{ job.scene_title || `Scene #${job.scene_id}` }}
                                    </span>
                                </div>
                                <p
                                    v-if="job.error_message"
                                    class="mt-0.5 truncate text-[10px] text-red-400/50"
                                    :title="job.error_message"
                                >
                                    {{ job.error_message }}
                                </p>
                            </div>
                            <button
                                class="border-border hover:border-lava/50 hover:text-lava shrink-0
                                    rounded border px-1.5 py-0.5 text-[10px] text-white/50
                                    transition-colors disabled:opacity-30"
                                :disabled="retryingJobId === job.job_id"
                                @click="handleRetryJob(job.job_id)"
                            >
                                {{ retryingJobId === job.job_id ? '...' : 'Retry' }}
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Active Jobs List -->
            <div class="px-4 py-3">
                <div class="mb-2 text-[10px] font-medium tracking-wider text-white/40 uppercase">
                    Active Jobs
                </div>

                <div v-if="jobStatusStore.activeJobs.length === 0" class="py-2">
                    <span class="text-xs text-white/40">No active jobs</span>
                </div>

                <div v-else class="space-y-2">
                    <div
                        v-for="job in jobStatusStore.activeJobs"
                        :key="job.job_id"
                        class="bg-void/50 flex items-center gap-2 rounded-md p-2"
                    >
                        <div class="min-w-0 flex-1">
                            <div class="flex items-center gap-1.5">
                                <Icon
                                    :name="phaseIcons[job.phase] ?? ''"
                                    size="10"
                                    class="text-lava shrink-0"
                                />
                                <span
                                    class="truncate text-xs text-white"
                                    :title="job.scene_title || `Scene #${job.scene_id}`"
                                >
                                    {{ job.scene_title || `Scene #${job.scene_id}` }}
                                </span>
                            </div>
                            <div class="mt-0.5 flex items-center gap-2">
                                <span class="text-[10px] text-white/40">
                                    {{ phaseLabels[job.phase] ?? job.phase }}
                                </span>
                                <span class="text-[10px] text-white/30">
                                    {{ formatElapsed(job.started_at) }}
                                </span>
                            </div>
                        </div>
                        <div
                            class="bg-emerald h-1.5 w-1.5 shrink-0 animate-pulse rounded-full"
                        ></div>
                    </div>

                    <div v-if="jobStatusStore.moreCount > 0" class="text-center">
                        <span class="text-[10px] text-white/40">
                            and {{ jobStatusStore.moreCount }} more...
                        </span>
                    </div>
                </div>
            </div>

            <!-- Footer -->
            <div class="border-border border-t px-4 py-2">
                <button
                    class="text-dim hover:text-lava flex w-full items-center justify-center gap-1.5
                        py-1 text-xs transition-colors"
                    @click="navigateToJobs"
                >
                    <span>View all jobs</span>
                    <Icon name="heroicons:arrow-right" size="12" />
                </button>
            </div>
        </div>
    </Teleport>
</template>
