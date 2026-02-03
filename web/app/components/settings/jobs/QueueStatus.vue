<script setup lang="ts">
defineProps<{
    poolConfig: { metadata_workers: number; thumbnail_workers: number; sprites_workers: number };
}>();

const jobStatusStore = useJobStatusStore();
const { phaseLabel, phaseIcon } = useJobFormatting();

const phases = ['metadata', 'thumbnail', 'sprites'] as const;

function phaseWaiting(phase: string): number {
    const p = jobStatusStore.byPhase[phase];
    return (p?.queued ?? 0) + (p?.pending ?? 0);
}
</script>

<template>
    <div class="glass-panel p-4">
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">Queue Status</h3>
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

        <div class="grid grid-cols-3 gap-3">
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
    </div>
</template>
