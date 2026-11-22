<script setup lang="ts">
import type { BulkCancelPhase, BulkCancelResponse } from '~/types/jobs';

const props = defineProps<{
    visible: boolean;
    phase: BulkCancelPhase | null;
}>();

const emit = defineEmits<{
    close: [];
    cancelled: [result: BulkCancelResponse];
}>();

const { cancelJobsBulk } = useApiJobs();
const jobStatusStore = useJobStatusStore();
const { phaseLabel } = useJobFormatting();

const includeRunning = ref(false);
const loading = ref(false);
const error = ref('');

// Live counts (SSE): pending in the database, queued in the worker pools, running.
const counts = computed(() => {
    if (!props.phase) return { pending: 0, queued: 0, running: 0 };
    if (props.phase === 'all') {
        return {
            pending: jobStatusStore.totalPending,
            queued: jobStatusStore.totalQueued,
            running: jobStatusStore.totalRunning,
        };
    }
    const p = jobStatusStore.byPhase[props.phase];
    return { pending: p?.pending ?? 0, queued: p?.queued ?? 0, running: p?.running ?? 0 };
});

const waiting = computed(() => counts.value.pending + counts.value.queued);
const total = computed(() => waiting.value + (includeRunning.value ? counts.value.running : 0));

const confirmLabel = computed(() => {
    if (total.value === 0) return 'Cancel jobs';
    return `Cancel ${total.value.toLocaleString()} job${total.value === 1 ? '' : 's'}`;
});

const title = computed(() =>
    props.phase === 'all' ? 'Cancel All Jobs' : `Cancel ${phaseLabel(props.phase ?? '')} Jobs`,
);

watch(
    () => props.visible,
    (open) => {
        if (open) {
            includeRunning.value = false;
            error.value = '';
        }
    },
);

const handleConfirm = async () => {
    if (!props.phase) return;
    error.value = '';
    loading.value = true;
    try {
        const result = await cancelJobsBulk(props.phase, includeRunning.value);
        emit('cancelled', result);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to cancel jobs';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    if (loading.value) return;
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && phase"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-2 text-sm font-semibold text-white">{{ title }}</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <p class="text-dim mb-3 text-xs">
                    All
                    <span class="text-white">{{ waiting.toLocaleString() }}</span>
                    waiting job{{ waiting === 1 ? '' : 's' }}
                    {{ phase === 'all' ? 'of every phase' : `of the ${phaseLabel(phase)} phase` }}
                    will be cancelled and will not run. Cancelled jobs stay in the history and can
                    be triggered again later.
                </p>

                <div
                    class="mb-3 grid grid-cols-3 gap-2 rounded-lg border border-white/5 bg-white/2
                        px-3 py-2 text-[11px]"
                >
                    <div>
                        <div class="text-dim">Pending</div>
                        <div class="font-medium text-amber-400">
                            {{ counts.pending.toLocaleString() }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim">Queued</div>
                        <div class="font-medium text-amber-400">
                            {{ counts.queued.toLocaleString() }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim">Running</div>
                        <div class="font-medium text-emerald-400">
                            {{ counts.running.toLocaleString() }}
                        </div>
                    </div>
                </div>

                <label class="mb-4 flex cursor-pointer items-start gap-2">
                    <input
                        v-model="includeRunning"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-3.5 w-3.5 rounded"
                        :disabled="loading"
                    />
                    <span class="text-xs text-white">
                        Also stop the running jobs
                        <span class="text-dim">({{ counts.running }})</span>
                        <span class="text-dim block text-[11px]">
                            Otherwise they finish normally.
                        </span>
                    </span>
                </label>

                <p class="text-dim mb-4 text-[11px]">
                    Counts are live and may change slightly before the cancel runs.
                </p>

                <div class="flex justify-end gap-2">
                    <button
                        :disabled="loading"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                        @click="handleClose"
                    >
                        Keep jobs
                    </button>
                    <button
                        :disabled="loading"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="handleConfirm"
                    >
                        {{ loading ? 'Cancelling...' : confirmLabel }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
