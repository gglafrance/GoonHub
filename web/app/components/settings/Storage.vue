<script setup lang="ts">
import type { StoragePath } from '~/types/storage';
import type { ScanHistory, ScanStatus, ScanProgressEvent } from '~/types/scan';

const {
    fetchStoragePaths,
    deleteStoragePath,
    startScan,
    cancelScan,
    getScanStatus,
    getScanHistory,
} = useApi();
const { message, error, clearMessages } = useSettingsMessage();
const authStore = useAuthStore();

const loading = ref(false);
const storagePaths = ref<StoragePath[]>([]);

// Modal state
const showModal = ref(false);
const editPath = ref<StoragePath | null>(null);

// Scan state
const scanLoading = ref(false);
const scanStatus = ref<ScanStatus>({ running: false });
const scanHistory = ref<ScanHistory[]>([]);
const scanHistoryTotal = ref(0);
const scanHistoryPage = ref(1);

// SSE for scan events
let eventSource: EventSource | null = null;

const loadStoragePaths = async () => {
    loading.value = true;
    clearMessages();
    try {
        const data = await fetchStoragePaths();
        storagePaths.value = data.storage_paths;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load storage paths';
    } finally {
        loading.value = false;
    }
};

const loadScanStatus = async () => {
    try {
        scanStatus.value = await getScanStatus();
    } catch (e: unknown) {
        console.error('Failed to load scan status:', e);
    }
};

const loadScanHistory = async () => {
    try {
        const data = await getScanHistory(scanHistoryPage.value, 5);
        scanHistory.value = data.data;
        scanHistoryTotal.value = data.total;
    } catch (e: unknown) {
        console.error('Failed to load scan history:', e);
    }
};

const handleStartScan = async () => {
    scanLoading.value = true;
    clearMessages();
    try {
        const scan = await startScan();
        scanStatus.value = { running: true, current_scan: scan };
        message.value = 'Scan started';
        connectSSE();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to start scan';
    } finally {
        scanLoading.value = false;
    }
};

const handleCancelScan = async () => {
    scanLoading.value = true;
    clearMessages();
    try {
        await cancelScan();
        message.value = 'Scan cancelled';
        await loadScanStatus();
        await loadScanHistory();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to cancel scan';
    } finally {
        scanLoading.value = false;
    }
};

const connectSSE = () => {
    if (!authStore.token || eventSource) return;

    const url = `/api/v1/events?token=${encodeURIComponent(authStore.token)}`;
    eventSource = new EventSource(url);

    eventSource.addEventListener('scan:progress', (e: MessageEvent) => {
        const event = JSON.parse(e.data);
        const data = event.data as ScanProgressEvent;
        if (scanStatus.value.current_scan && data) {
            scanStatus.value.current_scan.files_found = data.files_found;
            scanStatus.value.current_scan.videos_added = data.videos_added;
            scanStatus.value.current_scan.videos_skipped = data.videos_skipped;
            scanStatus.value.current_scan.errors = data.errors;
            scanStatus.value.current_scan.current_path = data.current_path;
            scanStatus.value.current_scan.current_file = data.current_file;
        }
    });

    eventSource.addEventListener('scan:video_added', () => {
        // Increment videos_added counter for immediate feedback
        if (scanStatus.value.current_scan) {
            scanStatus.value.current_scan.videos_added =
                (scanStatus.value.current_scan.videos_added || 0) + 1;
        }
    });

    eventSource.addEventListener('scan:completed', async () => {
        scanStatus.value = { running: false };
        message.value = 'Scan completed successfully';
        await loadScanHistory();
        disconnectSSE();
    });

    eventSource.addEventListener('scan:failed', async (e: MessageEvent) => {
        const event = JSON.parse(e.data);
        const data = event.data;
        scanStatus.value = { running: false };
        error.value = `Scan failed: ${data?.error_message || 'Unknown error'}`;
        await loadScanHistory();
        disconnectSSE();
    });

    eventSource.addEventListener('scan:cancelled', async () => {
        scanStatus.value = { running: false };
        message.value = 'Scan was cancelled';
        await loadScanHistory();
        disconnectSSE();
    });

    eventSource.onerror = () => {
        disconnectSSE();
    };
};

const disconnectSSE = () => {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
};

onMounted(async () => {
    await Promise.all([loadStoragePaths(), loadScanStatus(), loadScanHistory()]);
    if (scanStatus.value.running) {
        connectSSE();
    }
});

onUnmounted(() => {
    disconnectSSE();
});

const openCreate = () => {
    editPath.value = null;
    showModal.value = true;
};

const openEdit = (path: StoragePath) => {
    editPath.value = path;
    showModal.value = true;
};

const handleSaved = () => {
    showModal.value = false;
    message.value = editPath.value
        ? 'Storage path updated successfully'
        : 'Storage path created successfully';
    loadStoragePaths();
};

const handleDelete = async (path: StoragePath) => {
    if (!confirm(`Are you sure you want to delete "${path.name}"?`)) {
        return;
    }
    clearMessages();
    try {
        await deleteStoragePath(path.id);
        message.value = 'Storage path deleted successfully';
        loadStoragePaths();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete storage path';
    }
};

const formatDate = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    });
};

const formatDateTime = (dateStr: string): string => {
    const d = new Date(dateStr);
    return d.toLocaleString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
};

const formatDuration = (start: string, end?: string): string => {
    const startDate = new Date(start);
    const endDate = end ? new Date(end) : new Date();
    const diffMs = endDate.getTime() - startDate.getTime();
    const seconds = Math.floor(diffMs / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}m ${remainingSeconds}s`;
};

const getStatusColor = (status: string): string => {
    switch (status) {
        case 'completed':
            return 'bg-emerald/15 text-emerald border-emerald/30';
        case 'failed':
            return 'bg-lava/15 text-lava border-lava/30';
        case 'cancelled':
            return 'bg-amber-500/15 text-amber-500 border-amber-500/30';
        case 'running':
            return 'bg-blue-500/15 text-blue-500 border-blue-500/30';
        default:
            return 'bg-gray-500/15 text-gray-500 border-gray-500/30';
    }
};

const truncatePath = (path: string, maxLength = 50): string => {
    if (path.length <= maxLength) return path;
    return '...' + path.slice(-maxLength + 3);
};
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- Storage Paths -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <div>
                    <h3 class="text-sm font-semibold text-white">Storage Paths</h3>
                    <p class="text-dim mt-0.5 text-[11px]">
                        Configure where video files are stored. Mount external folders via Docker,
                        then register them here.
                    </p>
                </div>
                <button
                    @click="openCreate"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-[11px]
                        font-semibold text-white transition-all"
                >
                    Add Path
                </button>
            </div>

            <!-- Storage Paths Table -->
            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>
            <div v-else-if="storagePaths.length === 0" class="text-dim py-8 text-center text-xs">
                No storage paths configured
            </div>
            <div v-else class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                    <thead>
                        <tr
                            class="text-dim border-border border-b text-[11px] tracking-wider
                                uppercase"
                        >
                            <th class="pr-4 pb-2 font-medium">Name</th>
                            <th class="pr-4 pb-2 font-medium">Path</th>
                            <th class="pr-4 pb-2 font-medium">Default</th>
                            <th class="pr-4 pb-2 font-medium">Created</th>
                            <th class="pb-2 font-medium">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr
                            v-for="path in storagePaths"
                            :key="path.id"
                            class="border-border/50 border-b last:border-0"
                        >
                            <td class="py-2.5 pr-4 text-white">{{ path.name }}</td>
                            <td class="py-2.5 pr-4">
                                <code class="text-dim bg-void/50 rounded px-1.5 py-0.5 text-[11px]">
                                    {{ path.path }}
                                </code>
                            </td>
                            <td class="py-2.5 pr-4">
                                <span
                                    v-if="path.is_default"
                                    class="bg-emerald/15 text-emerald border-emerald/30 inline-block
                                        rounded-full border px-2 py-0.5 text-[10px] font-medium"
                                >
                                    Default
                                </span>
                            </td>
                            <td class="text-dim py-2.5 pr-4">{{ formatDate(path.created_at) }}</td>
                            <td class="py-2.5">
                                <div class="flex gap-2">
                                    <button
                                        @click="openEdit(path)"
                                        class="text-dim text-[11px] transition-colors
                                            hover:text-white"
                                    >
                                        Edit
                                    </button>
                                    <button
                                        v-if="storagePaths.length > 1"
                                        @click="handleDelete(path)"
                                        class="text-lava/70 hover:text-lava text-[11px]
                                            transition-colors"
                                    >
                                        Delete
                                    </button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Library Scan -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <div>
                    <h3 class="text-sm font-semibold text-white">Library Scan</h3>
                    <p class="text-dim mt-0.5 text-[11px]">
                        Scan storage paths to discover and import new video files automatically.
                    </p>
                </div>
                <div class="flex gap-2">
                    <button
                        v-if="!scanStatus.running"
                        @click="handleStartScan"
                        :disabled="scanLoading || storagePaths.length === 0"
                        class="bg-lava hover:bg-lava-glow disabled:bg-lava/50 rounded-lg px-3 py-1.5
                            text-[11px] font-semibold text-white transition-all
                            disabled:cursor-not-allowed"
                    >
                        {{ scanLoading ? 'Starting...' : 'Start Scan' }}
                    </button>
                    <button
                        v-else
                        @click="handleCancelScan"
                        :disabled="scanLoading"
                        class="rounded-lg border border-amber-500/30 bg-amber-500/15 px-3 py-1.5
                            text-[11px] font-semibold text-amber-500 transition-all
                            hover:bg-amber-500/25 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        {{ scanLoading ? 'Cancelling...' : 'Cancel Scan' }}
                    </button>
                </div>
            </div>

            <!-- Scan Progress (when running) -->
            <div
                v-if="scanStatus.running && scanStatus.current_scan"
                class="border-border mb-4 rounded-lg border bg-white/2 p-4"
            >
                <div class="mb-3 flex items-center gap-2">
                    <div class="h-2 w-2 animate-pulse rounded-full bg-blue-500"></div>
                    <span class="text-xs font-medium text-white">Scanning in progress...</span>
                </div>

                <div class="mb-3 grid grid-cols-4 gap-4">
                    <div>
                        <div class="text-dim text-[10px] tracking-wider uppercase">Found</div>
                        <div class="text-lg font-semibold text-white">
                            {{ scanStatus.current_scan.files_found }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim text-[10px] tracking-wider uppercase">Added</div>
                        <div class="text-emerald text-lg font-semibold">
                            {{ scanStatus.current_scan.videos_added }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim text-[10px] tracking-wider uppercase">Skipped</div>
                        <div class="text-dim text-lg font-semibold">
                            {{ scanStatus.current_scan.videos_skipped }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim text-[10px] tracking-wider uppercase">Errors</div>
                        <div
                            class="text-lg font-semibold"
                            :class="scanStatus.current_scan.errors > 0 ? 'text-lava' : 'text-dim'"
                        >
                            {{ scanStatus.current_scan.errors }}
                        </div>
                    </div>
                </div>

                <div v-if="scanStatus.current_scan.current_file" class="text-dim text-[11px]">
                    <span class="opacity-60">Current: </span>
                    <code class="bg-void/50 rounded px-1 py-0.5">
                        {{ truncatePath(scanStatus.current_scan.current_file, 60) }}
                    </code>
                </div>
            </div>

            <!-- Scan History -->
            <div v-if="scanHistory.length > 0">
                <h4 class="text-dim mb-2 text-[11px] font-medium tracking-wider uppercase">
                    Recent Scans
                </h4>
                <div class="overflow-x-auto">
                    <table class="w-full text-left text-xs">
                        <thead>
                            <tr class="text-dim border-border border-b text-[10px] uppercase">
                                <th class="pr-3 pb-2 font-medium">Date</th>
                                <th class="pr-3 pb-2 font-medium">Status</th>
                                <th class="pr-3 pb-2 font-medium">Found</th>
                                <th class="pr-3 pb-2 font-medium">Added</th>
                                <th class="pr-3 pb-2 font-medium">Duration</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr
                                v-for="scan in scanHistory"
                                :key="scan.id"
                                class="border-border/50 border-b last:border-0"
                            >
                                <td class="text-dim py-2 pr-3">
                                    {{ formatDateTime(scan.started_at) }}
                                </td>
                                <td class="py-2 pr-3">
                                    <span
                                        :class="getStatusColor(scan.status)"
                                        class="inline-block rounded-full border px-2 py-0.5
                                            text-[10px] font-medium capitalize"
                                    >
                                        {{ scan.status }}
                                    </span>
                                </td>
                                <td class="text-dim py-2 pr-3">{{ scan.files_found }}</td>
                                <td class="text-emerald py-2 pr-3">{{ scan.videos_added }}</td>
                                <td class="text-dim py-2 pr-3">
                                    {{ formatDuration(scan.started_at, scan.completed_at) }}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
            <div v-else class="text-dim py-4 text-center text-xs">No scan history</div>
        </div>

        <!-- Info Panel -->
        <div class="glass-panel p-5">
            <h3 class="mb-3 text-sm font-semibold text-white">About Storage Paths</h3>
            <div class="text-dim space-y-2 text-xs">
                <p>
                    Storage paths define where video files can be located. The system uses these
                    paths as reference locations - no files are copied between paths.
                </p>
                <p>
                    <strong class="text-white/80">To add external storage:</strong>
                </p>
                <ol class="list-inside list-decimal space-y-1 pl-2">
                    <li>Mount the external folder via Docker Compose volumes</li>
                    <li>
                        Click "Add Path" and enter the mounted path (e.g., /app/external/movies)
                    </li>
                    <li>The system will validate that the path exists and is accessible</li>
                    <li>Click "Start Scan" to discover and import videos from the new path</li>
                </ol>
                <p class="text-dim/70 mt-3 italic">
                    Note: The default storage path (./data/videos) is where uploaded videos are
                    stored.
                </p>
            </div>
        </div>

        <!-- Modal -->
        <SettingsStoragePathModal
            :visible="showModal"
            :storage-path="editPath"
            @close="showModal = false"
            @saved="handleSaved"
        />
    </div>
</template>
