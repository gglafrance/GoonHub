import type { JobStatusData, ActiveJobInfo, JobStatusPhase } from '~/types/jobs';

export const useJobStatusStore = defineStore('jobStatus', () => {
    const status = ref<JobStatusData | null>(null);
    const isConnected = ref(true);

    const totalRunning = computed(() => status.value?.total_running ?? 0);
    const totalQueued = computed(() => status.value?.total_queued ?? 0);
    const totalPending = computed(() => status.value?.total_pending ?? 0);
    // Total waiting includes both channel buffer (queued) and DB queue (pending)
    const totalWaiting = computed(() => totalQueued.value + totalPending.value);
    const isActive = computed(() => totalRunning.value > 0 || totalWaiting.value > 0);
    const activeJobs = computed<ActiveJobInfo[]>(() => status.value?.active_jobs ?? []);
    const byPhase = computed<Record<string, JobStatusPhase>>(() => status.value?.by_phase ?? {});
    const moreCount = computed(() => status.value?.more_count ?? 0);

    function updateStatus(newStatus: JobStatusData) {
        status.value = newStatus;
    }

    function setConnected(connected: boolean) {
        isConnected.value = connected;
    }

    return {
        status,
        isConnected,
        totalRunning,
        totalQueued,
        totalPending,
        totalWaiting,
        isActive,
        activeJobs,
        byPhase,
        moreCount,
        updateStatus,
        setConnected,
    };
});
