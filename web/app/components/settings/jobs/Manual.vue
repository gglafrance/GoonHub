<script setup lang="ts">
import type { ScanHistory, ScanStatus } from '~/types/scan';
import type { BulkJobResponse, PoolConfig } from '~/types/jobs';

const { startScan, cancelScan, getScanStatus, getScanHistory, triggerBulkPhase } = useApi();
const { fetchPoolConfig } = useApiJobs();
const { message, error, clearMessages } = useSettingsMessage();

// Duplication feature flag
const duplicationEnabled = ref(false);

type BulkPhase = 'metadata' | 'thumbnail' | 'sprites' | 'animated_thumbnails' | 'fingerprint';

const availablePhases = computed<BulkPhase[]>(() => {
    const phases: BulkPhase[] = ['metadata', 'thumbnail', 'sprites', 'animated_thumbnails'];
    if (duplicationEnabled.value) {
        phases.push('fingerprint');
    }
    return phases;
});

// Scan state
const scanLoading = ref(false);
const scanStatus = ref<ScanStatus>({ running: false });
const scanHistory = ref<ScanHistory[]>([]);
const scanHistoryTotal = ref(0);
const scanHistoryPage = ref(1);

// Bulk job state
const bulkLoading = ref<Record<string, boolean>>({
    metadata: false,
    thumbnail: false,
    sprites: false,
    animated_thumbnails: false,
    fingerprint: false,
});
const bulkResults = ref<Record<string, BulkJobResponse | null>>({
    metadata: null,
    thumbnail: null,
    sprites: null,
    animated_thumbnails: null,
    fingerprint: null,
});

const scanStore = useScanStore();

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
        message.value = 'Library scan started';
        scanStore.reset();
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

const handleBulkJob = async (
    phase: BulkPhase,
    mode: 'missing' | 'all',
) => {
    bulkLoading.value[phase] = true;
    bulkResults.value[phase] = null;
    clearMessages();
    try {
        const result = await triggerBulkPhase(phase, mode);
        bulkResults.value[phase] = result;
        message.value = `${phaseLabel(phase)} jobs queued: ${result.submitted} submitted, ${result.skipped} skipped`;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : `Failed to start ${phase} jobs`;
    } finally {
        bulkLoading.value[phase] = false;
    }
};

onMounted(async () => {
    await Promise.all([
        loadScanStatus(),
        loadScanHistory(),
        fetchPoolConfig().then((cfg: PoolConfig) => {
            duplicationEnabled.value = cfg.duplication_enabled;
        }).catch(() => {}),
    ]);
});

watch(
    () => scanStore.progress,
    (data) => {
        if (!data || !scanStatus.value.current_scan) return;
        scanStatus.value.current_scan.files_found = data.files_found;
        scanStatus.value.current_scan.videos_added = data.videos_added;
        scanStatus.value.current_scan.videos_skipped = data.videos_skipped;
        scanStatus.value.current_scan.videos_removed = data.videos_removed;
        scanStatus.value.current_scan.videos_moved = data.videos_moved;
        scanStatus.value.current_scan.errors = data.errors;
        scanStatus.value.current_scan.current_path = data.current_path;
        scanStatus.value.current_scan.current_file = data.current_file;
    },
);

watch(
    () => scanStore.completed,
    async (val) => {
        if (!val) return;
        scanStatus.value = { running: false };
        message.value = 'Library scan completed successfully';
        await loadScanHistory();
    },
);

watch(
    () => scanStore.failed,
    async (val) => {
        if (!val) return;
        scanStatus.value = { running: false };
        error.value = `Scan failed: ${scanStore.errorMessage || 'Unknown error'}`;
        await loadScanHistory();
    },
);

watch(
    () => scanStore.cancelled,
    async (val) => {
        if (!val) return;
        scanStatus.value = { running: false };
        message.value = 'Scan was cancelled';
        await loadScanHistory();
    },
);

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

const phaseLabel = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Metadata';
        case 'thumbnail':
            return 'Thumbnails';
        case 'sprites':
            return 'Sprites';
        case 'animated_thumbnails':
            return 'Animated Thumbnails';
        case 'fingerprint':
            return 'Fingerprint';
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
            return 'heroicons:film';
        case 'fingerprint':
            return 'heroicons:finger-print';
        default:
            return 'heroicons:cog-6-tooth';
    }
};

const phaseDescription = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Extract video metadata (duration, resolution, codecs)';
        case 'thumbnail':
            return 'Generate scene preview images and static marker thumbnails';
        case 'sprites':
            return 'Generate sprite sheets and VTT files for video preview';
        case 'animated_thumbnails':
            return 'Generate hover preview videos and animated marker clips';
        case 'fingerprint':
            return 'Generate audio/visual fingerprints for duplicate detection';
        default:
            return '';
    }
};
</script>

<template>
    <div class="space-y-5">
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

        <!-- Library Scan -->
        <div class="glass-panel p-5">
            <div class="mb-4 flex items-center justify-between">
                <div>
                    <div class="flex items-center gap-2">
                        <Icon name="heroicons:folder-open" size="16" class="text-lava" />
                        <h3 class="text-sm font-semibold text-white">Scan Library</h3>
                    </div>
                    <p class="text-dim mt-1 text-[11px]">
                        Scan storage paths to discover and import new video files automatically.
                    </p>
                </div>
                <div class="flex gap-2">
                    <button
                        v-if="!scanStatus.running"
                        :disabled="scanLoading"
                        class="bg-lava hover:bg-lava-glow disabled:bg-lava/50 rounded-lg px-3 py-1.5
                            text-[11px] font-semibold text-white transition-all
                            disabled:cursor-not-allowed"
                        @click="handleStartScan"
                    >
                        {{ scanLoading ? 'Starting...' : 'Start Scan' }}
                    </button>
                    <button
                        v-else
                        :disabled="scanLoading"
                        class="rounded-lg border border-amber-500/30 bg-amber-500/15 px-3 py-1.5
                            text-[11px] font-semibold text-amber-500 transition-all
                            hover:bg-amber-500/25 disabled:cursor-not-allowed disabled:opacity-50"
                        @click="handleCancelScan"
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

                <div class="mb-3 grid grid-cols-3 gap-4 sm:grid-cols-6">
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
                        <div class="text-dim text-[10px] tracking-wider uppercase">Moved</div>
                        <div
                            class="text-lg font-semibold"
                            :class="
                                scanStatus.current_scan.videos_moved > 0
                                    ? 'text-blue-400'
                                    : 'text-dim'
                            "
                        >
                            {{ scanStatus.current_scan.videos_moved || 0 }}
                        </div>
                    </div>
                    <div>
                        <div class="text-dim text-[10px] tracking-wider uppercase">Missing</div>
                        <div
                            class="text-lg font-semibold"
                            :class="
                                scanStatus.current_scan.videos_removed > 0
                                    ? 'text-amber-500'
                                    : 'text-dim'
                            "
                        >
                            {{ scanStatus.current_scan.videos_removed || 0 }}
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
                                <th class="pr-3 pb-2 font-medium">Moved</th>
                                <th class="pr-3 pb-2 font-medium">Missing</th>
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
                                <td
                                    class="py-2 pr-3"
                                    :class="scan.videos_moved > 0 ? 'text-blue-400' : 'text-dim'"
                                >
                                    {{ scan.videos_moved || 0 }}
                                </td>
                                <td
                                    class="py-2 pr-3"
                                    :class="scan.videos_removed > 0 ? 'text-amber-500' : 'text-dim'"
                                >
                                    {{ scan.videos_removed || 0 }}
                                </td>
                                <td class="text-dim py-2 pr-3">
                                    {{ formatDuration(scan.started_at, scan.completed_at) }}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
            <div v-else class="text-dim py-2 text-center text-xs">No scan history</div>
        </div>

        <!-- Bulk Processing Jobs -->
        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">Bulk Processing</h3>
                <p class="text-dim mt-1 text-[11px]">
                    Run processing jobs on multiple videos at once. Choose "Missing" to process only
                    videos that haven't been processed yet, or "All" to reprocess all videos.
                </p>
            </div>

            <div class="space-y-4">
                <div
                    v-for="phase in availablePhases"
                    :key="phase"
                    class="border-border rounded-lg border bg-white/2 p-4"
                >
                    <div class="flex items-start justify-between">
                        <div class="flex items-start gap-3">
                            <div
                                class="flex h-8 w-8 items-center justify-center rounded-lg
                                    bg-white/5"
                            >
                                <Icon :name="phaseIcon(phase)" size="16" class="text-dim" />
                            </div>
                            <div>
                                <h4 class="text-sm font-medium text-white">
                                    {{ phaseLabel(phase) }}
                                </h4>
                                <p class="text-dim mt-0.5 text-[11px]">
                                    {{ phaseDescription(phase) }}
                                </p>
                            </div>
                        </div>
                        <div class="flex items-center gap-2">
                            <button
                                :disabled="bulkLoading[phase]"
                                class="rounded-lg border border-white/10 bg-white/5 px-3 py-1.5
                                    text-[11px] font-medium text-white transition-all
                                    hover:bg-white/10 disabled:cursor-not-allowed
                                    disabled:opacity-50"
                                @click="handleBulkJob(phase, 'missing')"
                            >
                                {{ bulkLoading[phase] ? 'Queuing...' : 'Missing' }}
                            </button>
                            <button
                                :disabled="bulkLoading[phase]"
                                class="bg-lava/80 hover:bg-lava disabled:bg-lava/40 rounded-lg px-3
                                    py-1.5 text-[11px] font-medium text-white transition-all
                                    disabled:cursor-not-allowed"
                                @click="handleBulkJob(phase, 'all')"
                            >
                                {{ bulkLoading[phase] ? 'Queuing...' : 'All' }}
                            </button>
                        </div>
                    </div>

                    <!-- Result feedback -->
                    <div
                        v-if="bulkResults[phase]"
                        class="mt-3 flex items-center gap-4 rounded-lg bg-white/5 px-3 py-2
                            text-[11px]"
                    >
                        <div class="flex items-center gap-1.5">
                            <span class="text-dim">Submitted:</span>
                            <span class="text-emerald font-medium">{{
                                bulkResults[phase]?.submitted
                            }}</span>
                        </div>
                        <div class="flex items-center gap-1.5">
                            <span class="text-dim">Skipped:</span>
                            <span class="font-medium text-white">{{
                                bulkResults[phase]?.skipped
                            }}</span>
                        </div>
                        <div v-if="bulkResults[phase]?.errors" class="flex items-center gap-1.5">
                            <span class="text-dim">Errors:</span>
                            <span class="text-lava font-medium">{{
                                bulkResults[phase]?.errors
                            }}</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Info Panel -->
        <div class="glass-panel p-5">
            <h3 class="mb-3 text-sm font-semibold text-white">About Manual Jobs</h3>
            <div class="text-dim space-y-2 text-xs">
                <p>
                    <strong class="text-white/80">Scan Library:</strong> Discovers new video files
                    in your configured storage paths and adds them to the library.
                </p>
                <p>
                    <strong class="text-white/80">Metadata:</strong> Extracts video information like
                    duration, resolution, and codec details using ffprobe.
                </p>
                <p>
                    <strong class="text-white/80">Thumbnails:</strong> Generates scene preview
                    images at multiple resolutions and static marker thumbnails.
                </p>
                <p>
                    <strong class="text-white/80">Sprites:</strong> Creates sprite sheets and VTT
                    files for timeline preview on hover.
                </p>
                <p>
                    <strong class="text-white/80">Animated Thumbnails:</strong> Generates looping
                    video clips for marker previews.
                </p>
                <p v-if="duplicationEnabled">
                    <strong class="text-white/80">Fingerprint:</strong> Generates audio and visual
                    fingerprints for duplicate detection.
                </p>
                <p class="text-dim/70 mt-3 italic">
                    Jobs are queued and processed by background workers. Check the History tab to
                    monitor progress.
                </p>
            </div>
        </div>
    </div>
</template>
