<script setup lang="ts">
import type { HistoryPurgeInfo, HistoryPurgeResult } from '~/types/jobs';

const props = defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    purged: [result: HistoryPurgeResult];
}>();

const { fetchHistoryPurge, purgeHistory } = useApiJobs();

const info = ref<HistoryPurgeInfo | null>(null);
const loadingPreview = ref(false);
const purging = ref(false);
const error = ref('');

const preview = computed(() => info.value?.preview ?? null);
const settings = computed(() => info.value?.settings ?? null);

const loadPreview = async () => {
    loadingPreview.value = true;
    error.value = '';
    try {
        info.value = await fetchHistoryPurge();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to count purgeable jobs';
    } finally {
        loadingPreview.value = false;
    }
};

watch(
    () => props.visible,
    (open) => {
        if (open) {
            info.value = null;
            loadPreview();
        }
    },
);

const handleConfirm = async () => {
    purging.value = true;
    error.value = '';
    try {
        const response = await purgeHistory();
        emit('purged', response.result);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to purge job history';
    } finally {
        purging.value = false;
    }
};

const handleClose = () => {
    if (purging.value) return;
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-2 text-sm font-semibold text-white">Purge Job History</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <div v-if="loadingPreview" class="text-dim py-4 text-center text-xs">
                    Counting old jobs...
                </div>
                <template v-else-if="preview && settings">
                    <p class="text-dim mb-3 text-xs">
                        <template v-if="preview.total > 0">
                            <span class="text-white">{{ preview.total.toLocaleString() }}</span>
                            finished job record{{ preview.total === 1 ? '' : 's' }} will be
                            permanently deleted from the history. Pending and running jobs and the
                            dead letter queue are not touched. This cannot be undone.
                        </template>
                        <template v-else> No job record is old enough to be purged. </template>
                    </p>
                    <div
                        class="mb-4 space-y-1 rounded-lg border border-white/5 bg-white/2 px-3 py-2
                            text-[11px]"
                    >
                        <div class="flex justify-between">
                            <span class="text-dim">
                                Completed
                                <template v-if="settings.completed_enabled">
                                    (older than {{ settings.completed_retention }})
                                </template>
                            </span>
                            <span class="text-emerald-400">{{
                                preview.completed.toLocaleString()
                            }}</span>
                        </div>
                        <div class="flex justify-between">
                            <span class="text-dim">Cancelled</span>
                            <span class="text-amber-400">{{
                                preview.cancelled.toLocaleString()
                            }}</span>
                        </div>
                        <div class="flex justify-between">
                            <span class="text-dim">
                                Failed
                                <template v-if="settings.failed_enabled">
                                    (older than {{ settings.failed_retention }})
                                </template>
                            </span>
                            <span class="text-lava">{{ preview.failed.toLocaleString() }}</span>
                        </div>
                        <div class="flex justify-between">
                            <span class="text-dim">Timed out</span>
                            <span class="text-orange-400">{{
                                preview.timed_out.toLocaleString()
                            }}</span>
                        </div>
                    </div>
                </template>

                <div class="flex justify-end gap-2">
                    <button
                        :disabled="purging"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                        @click="handleClose"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="purging || loadingPreview || !preview || preview.total === 0"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="handleConfirm"
                    >
                        {{ purging ? 'Purging...' : 'Purge now' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
