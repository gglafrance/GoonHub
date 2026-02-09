<script setup lang="ts">
import type { DuplicateConfig, RescanStatus } from '~/types/duplicates';
import type { BulkJobResponse } from '~/types/jobs';

const { getConfig, updateConfig, startRescan, getRescanStatus } = useApiDuplicates();
const { triggerBulkPhase } = useApiJobs();

const loading = ref(true);
const saving = ref(false);
const error = ref('');
const success = ref('');
const config = ref<DuplicateConfig>({
    enabled: false,
    check_on_upload: true,
    match_threshold: 80,
    hamming_distance: 8,
    sample_interval: 2,
    duplicate_action: 'flag',
    keep_best_rules: ['duration', 'resolution', 'codec', 'bitrate'],
    keep_best_enabled: { duration: true, resolution: true, codec: true, bitrate: true },
    codec_preference: ['h265', 'hevc', 'av1', 'vp9', 'h264'],
});

const rescanStatus = ref<RescanStatus>({ running: false, total: 0, completed: 0, matched: 0 });
const rescanPolling = ref<ReturnType<typeof setInterval> | null>(null);

const indexingLoading = ref(false);
const indexingResult = ref<BulkJobResponse | null>(null);

async function startIndexing() {
    indexingLoading.value = true;
    indexingResult.value = null;
    error.value = '';
    try {
        indexingResult.value = await triggerBulkPhase('fingerprint', 'missing');
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to start fingerprint indexing';
    } finally {
        indexingLoading.value = false;
    }
}

const ruleLabels: Record<string, { label: string; description: string }> = {
    duration: { label: 'Duration', description: 'Prefer longer video' },
    resolution: { label: 'Resolution', description: 'Prefer higher resolution' },
    codec: { label: 'Codec', description: 'Prefer better codec' },
    bitrate: { label: 'Bitrate', description: 'Prefer higher bitrate' },
};

const rescanProgress = computed(() => {
    if (!rescanStatus.value.total) return 0;
    return Math.round((rescanStatus.value.completed / rescanStatus.value.total) * 100);
});

async function loadConfig() {
    loading.value = true;
    error.value = '';
    try {
        config.value = await getConfig();
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to load configuration';
    } finally {
        loading.value = false;
    }
}

async function saveConfig() {
    saving.value = true;
    error.value = '';
    success.value = '';
    try {
        config.value = await updateConfig(config.value);
        success.value = 'Configuration saved';
        setTimeout(() => (success.value = ''), 3000);
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to save configuration';
    } finally {
        saving.value = false;
    }
}

async function triggerRescan() {
    error.value = '';
    try {
        await startRescan();
        pollRescanStatus();
    } catch (e: unknown) {
        error.value = (e as Error).message || 'Failed to start rescan';
    }
}

async function pollRescanStatus() {
    if (rescanPolling.value) return;
    rescanPolling.value = setInterval(async () => {
        try {
            rescanStatus.value = await getRescanStatus();
            if (!rescanStatus.value.running && rescanPolling.value) {
                clearInterval(rescanPolling.value);
                rescanPolling.value = null;
            }
        } catch {
            if (rescanPolling.value) {
                clearInterval(rescanPolling.value);
                rescanPolling.value = null;
            }
        }
    }, 2000);
}

function moveRule(index: number, direction: -1 | 1) {
    const newIndex = index + direction;
    if (newIndex < 0 || newIndex >= config.value.keep_best_rules.length) return;
    const rules = [...config.value.keep_best_rules];
    [rules[index], rules[newIndex]] = [rules[newIndex], rules[index]];
    config.value.keep_best_rules = rules;
}

onMounted(async () => {
    await loadConfig();
    const status = await getRescanStatus();
    rescanStatus.value = status;
    if (status.running) pollRescanStatus();
});

onUnmounted(() => {
    if (rescanPolling.value) clearInterval(rescanPolling.value);
});
</script>

<template>
    <div class="glass-panel space-y-5 p-5">
        <div>
            <h3 class="text-sm font-semibold text-white">Duplicate Detection</h3>
            <p class="text-dim mt-1 text-[11px]">Configure automatic detection of duplicate videos using perceptual hashing.</p>
        </div>

        <div v-if="error" class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs">{{ error }}</div>
        <div v-if="success" class="rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs text-emerald-400">{{ success }}</div>

        <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="border-lava h-5 w-5 animate-spin rounded-full border-2 border-t-transparent" />
        </div>

        <template v-else>
            <!-- Enable Toggle -->
            <div class="flex items-center justify-between">
                <div>
                    <span class="text-xs font-medium text-white">Enable Detection</span>
                    <p class="text-dim text-[11px]">Automatically detect duplicates in your library</p>
                </div>
                <button
                    class="relative h-5 w-9 rounded-full transition-colors"
                    :class="config.enabled ? 'bg-emerald-500' : 'bg-white/10'"
                    @click="config.enabled = !config.enabled"
                >
                    <span
                        class="absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform"
                        :class="config.enabled ? 'translate-x-4' : 'translate-x-0.5'"
                    />
                </button>
            </div>

            <!-- Check on Upload -->
            <div class="border-border flex items-center justify-between border-t pt-4">
                <div>
                    <span class="text-xs font-medium text-white">Check on Upload</span>
                    <p class="text-dim text-[11px]">Automatically check new uploads for duplicates</p>
                </div>
                <button
                    class="relative h-5 w-9 rounded-full transition-colors"
                    :class="config.check_on_upload ? 'bg-emerald-500' : 'bg-white/10'"
                    @click="config.check_on_upload = !config.check_on_upload"
                >
                    <span
                        class="absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform"
                        :class="config.check_on_upload ? 'translate-x-4' : 'translate-x-0.5'"
                    />
                </button>
            </div>

            <!-- Thresholds Section -->
            <div class="border-border space-y-4 border-t pt-5">
                <div class="text-[11px] font-medium tracking-wider text-white/60 uppercase">Matching Thresholds</div>

                <!-- Match Threshold -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <span class="text-xs font-medium text-white">Match Threshold</span>
                        <span class="font-mono text-xs text-white/80">{{ config.match_threshold }}%</span>
                    </div>
                    <input
                        v-model.number="config.match_threshold"
                        type="range"
                        min="50"
                        max="100"
                        class="slider w-full"
                    />
                    <p class="text-dim text-[10px]">Minimum percentage of matching frames to flag as duplicate (50-100%)</p>
                </div>

                <!-- Hamming Distance -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <span class="text-xs font-medium text-white">Hamming Distance</span>
                        <span class="font-mono text-xs text-white/80">{{ config.hamming_distance }}</span>
                    </div>
                    <input
                        v-model.number="config.hamming_distance"
                        type="range"
                        min="1"
                        max="15"
                        class="slider w-full"
                    />
                    <p class="text-dim text-[10px]">Maximum bit difference between frame hashes to consider a match (1-15, lower = stricter)</p>
                </div>

                <!-- Sample Interval -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <span class="text-xs font-medium text-white">Sample Interval</span>
                        <span class="font-mono text-xs text-white/80">{{ config.sample_interval }}s</span>
                    </div>
                    <input
                        v-model.number="config.sample_interval"
                        type="range"
                        min="1"
                        max="10"
                        class="slider w-full"
                    />
                    <p class="text-dim text-[10px]">Seconds between sampled frames. Lower = more precise, higher = faster. Changing requires re-fingerprinting.</p>
                </div>
            </div>

            <!-- Duplicate Action -->
            <div class="border-border space-y-3 border-t pt-5">
                <div class="text-[11px] font-medium tracking-wider text-white/60 uppercase">On Duplicate Found</div>
                <div class="border-border bg-panel flex items-center rounded-lg border p-0.5">
                    <button
                        v-for="action in ['flag', 'mark', 'trash']"
                        :key="action"
                        class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                        :class="config.duplicate_action === action
                            ? 'bg-lava/15 text-lava'
                            : 'text-dim hover:text-white'"
                        @click="config.duplicate_action = action as 'flag' | 'mark' | 'trash'"
                    >
                        {{ action === 'flag' ? 'Flag Only' : action === 'mark' ? 'Mark & Flag' : 'Move to Trash' }}
                    </button>
                </div>
            </div>

            <!-- Keep-Best Rules -->
            <div class="border-border space-y-3 border-t pt-5">
                <div class="text-[11px] font-medium tracking-wider text-white/60 uppercase">Keep Best Rules</div>
                <p class="text-dim text-[11px]">Rules are applied in order. First rule that breaks a tie wins.</p>
                <div class="space-y-1">
                    <div
                        v-for="(rule, index) in config.keep_best_rules"
                        :key="rule"
                        class="border-border bg-surface flex items-center gap-2 rounded-lg border px-3 py-2"
                    >
                        <div class="flex flex-col gap-0.5">
                            <button
                                class="text-dim hover:text-white disabled:opacity-20"
                                :disabled="index === 0"
                                @click="moveRule(index, -1)"
                            >
                                <Icon name="heroicons:chevron-up" class="h-3 w-3" />
                            </button>
                            <button
                                class="text-dim hover:text-white disabled:opacity-20"
                                :disabled="index === config.keep_best_rules.length - 1"
                                @click="moveRule(index, 1)"
                            >
                                <Icon name="heroicons:chevron-down" class="h-3 w-3" />
                            </button>
                        </div>
                        <div class="flex-1">
                            <span class="text-xs font-medium text-white">{{ ruleLabels[rule]?.label || rule }}</span>
                            <p class="text-dim text-[10px]">{{ ruleLabels[rule]?.description || '' }}</p>
                        </div>
                        <button
                            class="relative h-4 w-7 rounded-full transition-colors"
                            :class="config.keep_best_enabled[rule] ? 'bg-emerald-500' : 'bg-white/10'"
                            @click="config.keep_best_enabled[rule] = !config.keep_best_enabled[rule]"
                        >
                            <span
                                class="absolute top-0.5 h-3 w-3 rounded-full bg-white transition-transform"
                                :class="config.keep_best_enabled[rule] ? 'translate-x-3' : 'translate-x-0.5'"
                            />
                        </button>
                    </div>
                </div>
            </div>

            <!-- Save Button -->
            <div class="border-border flex items-center justify-between border-t pt-4">
                <button
                    class="bg-lava hover:bg-lava/90 rounded-lg px-4 py-1.5 text-xs font-medium text-white transition-colors disabled:opacity-50"
                    :disabled="saving"
                    @click="saveConfig"
                >
                    {{ saving ? 'Saving...' : 'Save Configuration' }}
                </button>
            </div>

            <!-- Rescan Section -->
            <div class="border-border space-y-3 border-t pt-5">
                <div class="text-[11px] font-medium tracking-wider text-white/60 uppercase">Library Rescan</div>
                <p class="text-dim text-[11px]">Re-scan all fingerprinted scenes for duplicates using current settings.</p>

                <div v-if="rescanStatus.running" class="space-y-2">
                    <div class="flex items-center justify-between text-xs">
                        <span class="text-white">Scanning...</span>
                        <span class="font-mono text-white/60">{{ rescanStatus.completed }} / {{ rescanStatus.total }}</span>
                    </div>
                    <div class="h-1.5 w-full overflow-hidden rounded-full bg-white/10">
                        <div
                            class="bg-lava h-full rounded-full transition-all"
                            :style="{ width: rescanProgress + '%' }"
                        />
                    </div>
                    <div class="text-dim text-[10px]">{{ rescanStatus.matched }} matches found</div>
                </div>

                <button
                    v-else
                    class="border-border hover:border-lava/30 hover:text-lava text-dim rounded-lg border px-4 py-1.5 text-xs font-medium transition-colors"
                    @click="triggerRescan"
                >
                    Start Library Rescan
                </button>
            </div>

            <!-- Fingerprint Indexing Section -->
            <div class="border-border space-y-3 border-t pt-5">
                <div class="text-[11px] font-medium tracking-wider text-white/60 uppercase">Fingerprint Indexing</div>
                <p class="text-dim text-[11px]">Generate fingerprints for existing videos that haven't been indexed yet. Required before duplicate detection can identify them.</p>

                <button
                    :disabled="indexingLoading"
                    class="border-border hover:border-lava/30 hover:text-lava text-dim rounded-lg border px-4 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                    @click="startIndexing"
                >
                    {{ indexingLoading ? 'Queuing...' : 'Start Indexing' }}
                </button>

                <div
                    v-if="indexingResult"
                    class="flex items-center gap-4 rounded-lg bg-white/5 px-3 py-2 text-[11px]"
                >
                    <div class="flex items-center gap-1.5">
                        <span class="text-dim">Submitted:</span>
                        <span class="text-emerald font-medium">{{ indexingResult.submitted }}</span>
                    </div>
                    <div class="flex items-center gap-1.5">
                        <span class="text-dim">Skipped:</span>
                        <span class="font-medium text-white">{{ indexingResult.skipped }}</span>
                    </div>
                    <div v-if="indexingResult.errors" class="flex items-center gap-1.5">
                        <span class="text-dim">Errors:</span>
                        <span class="text-lava font-medium">{{ indexingResult.errors }}</span>
                    </div>
                </div>
            </div>
        </template>
    </div>
</template>

<style scoped>
.slider {
    -webkit-appearance: none;
    appearance: none;
    height: 4px;
    border-radius: 2px;
    background: rgba(255, 255, 255, 0.1);
    outline: none;
}
.slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: #ff4d4d;
    border: 2px solid rgba(0, 0, 0, 0.3);
    cursor: pointer;
}
.slider::-moz-range-thumb {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: #ff4d4d;
    border: 2px solid rgba(0, 0, 0, 0.3);
    cursor: pointer;
}
</style>
