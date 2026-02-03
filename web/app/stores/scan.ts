import type { ScanProgressEvent } from '~/types/scan';

export const useScanStore = defineStore('scan', () => {
    const progress = ref<ScanProgressEvent | null>(null);
    const completed = ref(false);
    const failed = ref(false);
    const cancelled = ref(false);
    const errorMessage = ref('');

    function updateProgress(data: ScanProgressEvent) {
        progress.value = data;
        completed.value = false;
        failed.value = false;
        cancelled.value = false;
    }

    function markCompleted() {
        completed.value = true;
        progress.value = null;
    }

    function markFailed(error: string) {
        failed.value = true;
        errorMessage.value = error;
        progress.value = null;
    }

    function markCancelled() {
        cancelled.value = true;
        progress.value = null;
    }

    function reset() {
        progress.value = null;
        completed.value = false;
        failed.value = false;
        cancelled.value = false;
        errorMessage.value = '';
    }

    return {
        progress,
        completed,
        failed,
        cancelled,
        errorMessage,
        updateProgress,
        markCompleted,
        markFailed,
        markCancelled,
        reset,
    };
});
