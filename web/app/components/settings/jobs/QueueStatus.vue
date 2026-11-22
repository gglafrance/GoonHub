<script setup lang="ts">
import type { BulkCancelPhase, BulkCancelResponse } from '~/types/jobs';

defineProps<{
    poolConfig: {
        metadata_workers: number;
        thumbnail_workers: number;
        sprites_workers: number;
        animated_thumbnails_workers: number;
        fingerprint_workers: number;
    };
}>();

const emit = defineEmits<{
    cancelled: [result: BulkCancelResponse];
}>();

const jobStatusStore = useJobStatusStore();
const { phaseLabel, phaseIcon } = useJobFormatting();

const phases = ['metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint'] as const;

const cancelPhase = ref<BulkCancelPhase | null>(null);

function phaseWaiting(phase: string): number {
    const p = jobStatusStore.byPhase[phase];
    return (p?.queued ?? 0) + (p?.pending ?? 0);
}

function phaseHasJobs(phase: string): boolean {
    return phaseWaiting(phase) > 0 || (jobStatusStore.byPhase[phase]?.running ?? 0) > 0;
}

const handleCancelled = (result: BulkCancelResponse) => {
    cancelPhase.value = null;
    emit('cancelled', result);
};
</script>

<template>
    <div class="glass-panel p-4">
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">Queue Status</h3>
            <div class="flex items-center gap-3">
                <button
                    v-if="jobStatusStore.isActive"
                    class="text-dim hover:border-lava/30 hover:text-lava flex items-center gap-1
                        rounded border border-white/10 px-2 py-0.5 text-[10px] font-medium
                        transition-colors"
                    title="Cancel the jobs of every phase"
                    @click="cancelPhase = 'all'"
                >
                    <Icon name="heroicons:x-circle" size="11" />
                    Cancel all
                </button>
                <div
                    v-if="!jobStatusStore.isConnected"
                    class="flex items-center gap-1.5 text-amber-500"
                >
                    <Icon name="heroicons:exclamation-triangle" size="12" />
                    <span class="text-[10px]">Disconnected</span>
                </div>
                <div v-else class="flex items-center gap-1.5 text-emerald-500">
                    <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></span>
                    <span class="text-[10px]">Live</span>
                </div>
            </div>
        </div>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-5">
            <div
                v-for="phase in phases"
                :key="phase"
                class="rounded-lg border border-white/5 bg-white/2 px-3 py-2.5"
            >
                <div
                    class="text-dim mb-2 flex items-center gap-1.5 text-[10px] font-medium
                        tracking-wider uppercase"
                >
                    <Icon :name="phaseIcon(phase)" size="11" />
                    {{ phaseLabel(phase) }}
                    <button
                        v-if="phaseHasJobs(phase)"
                        class="hover:text-lava ml-auto rounded p-0.5 text-white/30
                            transition-colors"
                        :title="`Cancel all ${phaseLabel(phase)} jobs`"
                        :aria-label="`Cancel all ${phaseLabel(phase)} jobs`"
                        @click="cancelPhase = phase"
                    >
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </div>
                <div class="flex items-baseline gap-3">
                    <div class="flex items-baseline gap-1" title="Workers">
                        <span class="text-dim text-[10px]">W</span>
                        <span class="text-xs font-medium text-white">{{
                            poolConfig[`${phase}_workers` as keyof typeof poolConfig]
                        }}</span>
                    </div>
                    <div class="flex items-baseline gap-1" title="Running">
                        <span class="text-[10px] text-emerald-400">R</span>
                        <span
                            class="text-xs font-medium"
                            :class="
                                (jobStatusStore.byPhase[phase]?.running ?? 0) > 0
                                    ? 'text-emerald-400'
                                    : 'text-dim'
                            "
                            >{{ jobStatusStore.byPhase[phase]?.running ?? 0 }}</span
                        >
                    </div>
                    <div class="flex items-baseline gap-1" title="Pending (queued + DB)">
                        <span class="text-[10px] text-amber-400">P</span>
                        <span
                            class="text-xs font-medium"
                            :class="phaseWaiting(phase) > 0 ? 'text-amber-400' : 'text-dim'"
                            >{{ phaseWaiting(phase) }}</span
                        >
                    </div>
                </div>
            </div>
        </div>

        <SettingsJobsBulkCancelModal
            :visible="cancelPhase !== null"
            :phase="cancelPhase"
            @close="cancelPhase = null"
            @cancelled="handleCancelled"
        />
    </div>
</template>
