<script setup lang="ts">
import type { TranscodeCacheStats } from '~/types/admin';

// Size of the cache of on-the-fly conversions, with a button to clear it.
const { getTranscodeCache, clearTranscodeCache } = useApiAdmin();
const { formatSize } = useFormatter();
const { message, error, setMessage, setError } = useSettingsMessage();

const stats = ref<TranscodeCacheStats | null>(null);
const loading = ref(false);
const showConfirm = ref(false);
const clearing = ref(false);

const usedPercent = computed(() => {
    const s = stats.value;
    if (!s || s.max_size_bytes <= 0) return 0;
    return Math.min(100, Math.round((s.size_bytes / s.max_size_bytes) * 100));
});

const maxAgeLabel = computed(() => {
    const seconds = stats.value?.max_age_seconds ?? 0;
    if (seconds >= 86400 && seconds % 86400 === 0) return `${seconds / 86400} d`;
    if (seconds >= 3600) return `${Math.round(seconds / 3600)} h`;
    return `${Math.round(seconds / 60)} min`;
});

const load = async () => {
    loading.value = true;
    try {
        stats.value = await getTranscodeCache();
    } catch (e) {
        setError(e, 'Failed to load the transcode cache');
    } finally {
        loading.value = false;
    }
};

const clear = async () => {
    clearing.value = true;
    try {
        const res = await clearTranscodeCache();
        stats.value = res;
        setMessage(
            `Removed ${res.removed} conversion${res.removed === 1 ? '' : 's'} (${formatSize(res.removed_bytes)})`,
        );
        showConfirm.value = false;
    } catch (e) {
        setError(e, 'Failed to clear the transcode cache');
    } finally {
        clearing.value = false;
    }
};

onMounted(load);
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-2 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-white">Transcode Cache</h3>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="loading"
                title="Refresh"
                @click="load"
            >
                <Icon name="heroicons:arrow-path" size="14" :class="{ 'animate-spin': loading }" />
            </button>
        </div>
        <p class="text-dim mb-4 text-xs">
            What on-the-fly conversions produced is kept, so that replays, seeks back and other
            viewers of the same part don't run ffmpeg again. Least recently used conversions are
            removed above the size limit or after the max age.
        </p>

        <div v-if="stats && !stats.enabled" class="text-dim text-xs">
            The cache is turned off in the server configuration (<code class="text-white"
                >streaming.transcode.cache.enabled</code
            >).
        </div>

        <div v-else-if="stats" class="space-y-3">
            <div class="grid grid-cols-3 gap-3">
                <div class="border-border rounded-lg border bg-white/2 px-3 py-2">
                    <div class="text-dim text-[11px] tracking-wide uppercase">Size</div>
                    <div class="text-sm font-semibold text-white">
                        {{ formatSize(stats.size_bytes) }}
                        <span class="text-dim text-xs font-normal">
                            / {{ formatSize(stats.max_size_bytes) }}
                        </span>
                    </div>
                </div>
                <div class="border-border rounded-lg border bg-white/2 px-3 py-2">
                    <div class="text-dim text-[11px] tracking-wide uppercase">Conversions</div>
                    <div class="text-sm font-semibold text-white">
                        {{ stats.entries }}
                        <span v-if="stats.in_progress" class="text-xs font-normal text-emerald-400">
                            {{ stats.in_progress }} running
                        </span>
                    </div>
                </div>
                <div class="border-border rounded-lg border bg-white/2 px-3 py-2">
                    <div class="text-dim text-[11px] tracking-wide uppercase">Max age</div>
                    <div class="text-sm font-semibold text-white">{{ maxAgeLabel }}</div>
                </div>
            </div>

            <div class="h-1 overflow-hidden rounded-full bg-white/5">
                <div class="bg-lava h-full transition-all" :style="{ width: `${usedPercent}%` }" />
            </div>

            <div class="flex items-center justify-between gap-3">
                <p class="text-xs">
                    <span v-if="message" class="text-emerald-400">{{ message }}</span>
                    <span v-else-if="error" class="text-red-400">{{ error }}</span>
                </p>
                <button
                    :disabled="stats.entries === 0"
                    class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                        transition-colors hover:border-red-500/50 hover:text-red-400
                        disabled:cursor-not-allowed disabled:opacity-40"
                    @click="showConfirm = true"
                >
                    Clear cache
                </button>
            </div>
        </div>

        <p v-else-if="error" class="text-xs text-red-400">{{ error }}</p>

        <SettingsAppTranscodeCacheClearModal
            :visible="showConfirm"
            :count="stats?.entries ?? 0"
            :size="formatSize(stats?.size_bytes ?? 0)"
            :loading="clearing"
            @close="showConfirm = false"
            @confirm="clear"
        />
    </div>
</template>
