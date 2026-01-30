<script setup lang="ts">
import type { QueueStatus } from '~/types/jobs';

defineProps<{
    queueStatus: QueueStatus;
    poolConfig: { metadata_workers: number; thumbnail_workers: number; sprites_workers: number };
    autoRefresh: boolean;
}>();

const emit = defineEmits<{
    toggleAutoRefresh: [];
    refresh: [];
}>();

const { phaseLabel, phaseIcon } = useJobFormatting();
</script>

<template>
    <div class="glass-panel p-4">
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">Queue Status</h3>
            <div class="flex items-center gap-3">
                <label class="flex cursor-pointer items-center gap-1.5">
                    <span class="text-dim text-[11px]">Auto</span>
                    <button
                        @click="emit('toggleAutoRefresh')"
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
                    @click="emit('refresh')"
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
                                queueStatus[`${phase}_running` as keyof typeof queueStatus] > 0
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
                                queueStatus[`${phase}_queued` as keyof typeof queueStatus] > 0
                                    ? 'text-amber-400'
                                    : 'text-dim'
                            "
                            >{{ queueStatus[`${phase}_queued` as keyof typeof queueStatus] }}</span
                        >
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
