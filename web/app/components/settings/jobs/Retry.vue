<script setup lang="ts">
import type { RetryConfig } from '~/types/jobs';

const { fetchRetryConfig, updateRetryConfig } = useApi();

const loading = ref(true);
const saving = ref<string | null>(null);
const error = ref('');
const message = ref('');

const configs = ref<RetryConfig[]>([]);

const loadConfig = async () => {
    loading.value = true;
    error.value = '';
    try {
        configs.value = await fetchRetryConfig();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load retry config';
    } finally {
        loading.value = false;
    }
};

const saveConfig = async (config: RetryConfig) => {
    saving.value = config.phase;
    error.value = '';
    message.value = '';
    try {
        await updateRetryConfig({
            phase: config.phase,
            max_retries: config.max_retries,
            initial_delay_seconds: config.initial_delay_seconds,
            max_delay_seconds: config.max_delay_seconds,
            backoff_factor: config.backoff_factor,
        });
        message.value = `Retry config for ${config.phase} updated`;
        setTimeout(() => {
            message.value = '';
        }, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update retry config';
    } finally {
        saving.value = null;
    }
};

const clampRetries = (val: number) => Math.max(0, Math.min(10, val));
const clampDelay = (val: number) => Math.max(1, Math.min(86400, val));
const clampBackoff = (val: number) => Math.max(1.0, Math.min(5.0, val));

const phaseIcon = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'heroicons:document-text';
        case 'thumbnail':
            return 'heroicons:photo';
        case 'sprites':
            return 'heroicons:squares-2x2';
        case 'scan':
            return 'heroicons:folder-open';
        default:
            return 'heroicons:cog-6-tooth';
    }
};

const phaseDescription = (phase: string): string => {
    switch (phase) {
        case 'metadata':
            return 'Video duration and resolution extraction';
        case 'thumbnail':
            return 'Preview thumbnail generation';
        case 'sprites':
            return 'Sprite sheets and VTT files';
        case 'scan':
            return 'Library scan operations';
        default:
            return '';
    }
};

const formatDelay = (seconds: number): string => {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    return `${Math.floor(seconds / 3600)}h`;
};

onMounted(() => {
    loadConfig();
});
</script>

<template>
    <div class="space-y-5">
        <div class="glass-panel p-5">
            <div class="mb-4">
                <h3 class="text-sm font-semibold text-white">Retry Configuration</h3>
                <p class="text-dim mt-1 text-[11px]">
                    Configure automatic retry behavior for failed jobs. Jobs that exhaust all
                    retries are moved to the Dead Letter Queue.
                </p>
            </div>

            <div
                v-if="error"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ error }}
            </div>

            <div
                v-if="message"
                class="mb-4 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2
                    text-xs text-emerald-400"
            >
                {{ message }}
            </div>

            <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

            <div v-else class="space-y-6">
                <div
                    v-for="config in configs"
                    :key="config.phase"
                    class="rounded-lg border border-white/5 bg-white/2 p-4"
                >
                    <!-- Phase Header -->
                    <div class="mb-4 flex items-center justify-between">
                        <div class="flex items-center gap-2">
                            <Icon :name="phaseIcon(config.phase)" size="14" class="text-dim" />
                            <div>
                                <span class="text-xs font-medium text-white capitalize">
                                    {{ config.phase }}
                                </span>
                                <p class="text-dim text-[10px]">
                                    {{ phaseDescription(config.phase) }}
                                </p>
                            </div>
                        </div>
                        <button
                            @click="saveConfig(config)"
                            :disabled="saving === config.phase"
                            class="rounded-lg bg-white/5 px-3 py-1 text-[11px] font-medium
                                text-white transition-colors hover:bg-white/10 disabled:opacity-50"
                        >
                            {{ saving === config.phase ? 'Saving...' : 'Save' }}
                        </button>
                    </div>

                    <!-- Config Fields -->
                    <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
                        <!-- Max Retries -->
                        <div>
                            <label class="text-dim mb-1 block text-[10px] font-medium">
                                Max Retries
                            </label>
                            <input
                                v-model.number="config.max_retries"
                                @change="config.max_retries = clampRetries(config.max_retries)"
                                type="number"
                                min="0"
                                max="10"
                                class="border-border bg-surface w-full rounded-lg border px-3 py-1.5
                                    text-xs text-white focus:border-white/20 focus:outline-none"
                            />
                            <span class="text-dim text-[9px]">0 = no retries</span>
                        </div>

                        <!-- Initial Delay -->
                        <div>
                            <label class="text-dim mb-1 block text-[10px] font-medium">
                                Initial Delay (s)
                            </label>
                            <input
                                v-model.number="config.initial_delay_seconds"
                                @change="
                                    config.initial_delay_seconds = clampDelay(
                                        config.initial_delay_seconds,
                                    )
                                "
                                type="number"
                                min="1"
                                max="3600"
                                class="border-border bg-surface w-full rounded-lg border px-3 py-1.5
                                    text-xs text-white focus:border-white/20 focus:outline-none"
                            />
                            <span class="text-dim text-[9px]">
                                {{ formatDelay(config.initial_delay_seconds) }} before first retry
                            </span>
                        </div>

                        <!-- Max Delay -->
                        <div>
                            <label class="text-dim mb-1 block text-[10px] font-medium">
                                Max Delay (s)
                            </label>
                            <input
                                v-model.number="config.max_delay_seconds"
                                @change="
                                    config.max_delay_seconds = clampDelay(config.max_delay_seconds)
                                "
                                type="number"
                                min="1"
                                max="86400"
                                class="border-border bg-surface w-full rounded-lg border px-3 py-1.5
                                    text-xs text-white focus:border-white/20 focus:outline-none"
                            />
                            <span class="text-dim text-[9px]">
                                Cap at {{ formatDelay(config.max_delay_seconds) }}
                            </span>
                        </div>

                        <!-- Backoff Factor -->
                        <div>
                            <label class="text-dim mb-1 block text-[10px] font-medium">
                                Backoff Factor
                            </label>
                            <input
                                v-model.number="config.backoff_factor"
                                @change="
                                    config.backoff_factor = clampBackoff(config.backoff_factor)
                                "
                                type="number"
                                min="1.0"
                                max="5.0"
                                step="0.1"
                                class="border-border bg-surface w-full rounded-lg border px-3 py-1.5
                                    text-xs text-white focus:border-white/20 focus:outline-none"
                            />
                            <span class="text-dim text-[9px]">Exponential multiplier</span>
                        </div>
                    </div>

                    <!-- Preview -->
                    <div class="mt-3 rounded-lg bg-white/2 p-2">
                        <span class="text-dim text-[10px]">Retry delays: </span>
                        <span class="text-[10px] text-white">
                            <template
                                v-for="(_, i) in Array(Math.min(config.max_retries, 5))"
                                :key="i"
                            >
                                {{
                                    formatDelay(
                                        Math.min(
                                            Math.round(
                                                config.initial_delay_seconds *
                                                    Math.pow(config.backoff_factor, i),
                                            ),
                                            config.max_delay_seconds,
                                        ),
                                    )
                                }}{{ i < Math.min(config.max_retries, 5) - 1 ? ' → ' : '' }}
                            </template>
                            <template v-if="config.max_retries > 5">...</template>
                            <template v-if="config.max_retries === 0">
                                <span class="text-dim">No retries configured</span>
                            </template>
                        </span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Info Panel -->
        <div class="glass-panel p-4">
            <div class="flex items-start gap-3">
                <Icon name="heroicons:information-circle" size="16" class="text-dim mt-0.5" />
                <div class="text-dim space-y-1 text-[11px]">
                    <p>
                        <strong class="text-white">Exponential Backoff:</strong> Each retry waits
                        longer than the previous. Delay = Initial × Factor^(retry number).
                    </p>
                    <p>
                        <strong class="text-white">Dead Letter Queue:</strong> After max retries are
                        exhausted, jobs are moved to the DLQ for manual review.
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>
