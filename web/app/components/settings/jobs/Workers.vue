<script setup lang="ts">
import type { PoolConfig } from '~/types/jobs';

const { fetchPoolConfig, updatePoolConfig } = useApi();

const loading = ref(true);
const saving = ref(false);
const error = ref('');
const message = ref('');

const metadataWorkers = ref(3);
const thumbnailWorkers = ref(1);
const spritesWorkers = ref(1);

const loadConfig = async () => {
    loading.value = true;
    error.value = '';
    try {
        const config: PoolConfig = await fetchPoolConfig();
        metadataWorkers.value = config.metadata_workers;
        thumbnailWorkers.value = config.thumbnail_workers;
        spritesWorkers.value = config.sprites_workers;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load pool config';
    } finally {
        loading.value = false;
    }
};

const applyConfig = async () => {
    saving.value = true;
    error.value = '';
    message.value = '';
    try {
        await updatePoolConfig({
            metadata_workers: metadataWorkers.value,
            thumbnail_workers: thumbnailWorkers.value,
            sprites_workers: spritesWorkers.value,
        });
        message.value = 'Pool configuration updated';
        setTimeout(() => {
            message.value = '';
        }, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update pool config';
    } finally {
        saving.value = false;
    }
};

const clamp = (val: number) => Math.max(1, Math.min(10, val));

onMounted(() => {
    loadConfig();
});
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4">
            <h3 class="text-sm font-semibold text-white">Worker Pool Configuration</h3>
            <p class="text-dim mt-1 text-[11px]">
                Configure the number of concurrent workers for each processing phase. Changes take
                effect immediately.
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
            class="mb-4 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs
                text-emerald-400"
        >
            {{ message }}
        </div>

        <div v-if="loading" class="text-dim py-8 text-center text-xs">Loading...</div>

        <div v-else class="space-y-4">
            <!-- Metadata Workers -->
            <div class="flex items-center justify-between">
                <div>
                    <label class="flex items-center gap-1.5 text-xs font-medium text-white">
                        <Icon name="heroicons:document-text" size="13" class="text-dim" />
                        Metadata
                    </label>
                    <p class="text-dim text-[10px]">
                        Extracts video duration, resolution, codec info
                    </p>
                </div>
                <input
                    v-model.number="metadataWorkers"
                    @change="metadataWorkers = clamp(metadataWorkers)"
                    type="number"
                    min="1"
                    max="10"
                    class="border-border bg-surface w-16 rounded-lg border px-2 py-1.5 text-center
                        text-xs text-white focus:border-white/20 focus:outline-none"
                />
            </div>

            <!-- Thumbnail Workers -->
            <div class="flex items-center justify-between">
                <div>
                    <label class="flex items-center gap-1.5 text-xs font-medium text-white">
                        <Icon name="heroicons:photo" size="13" class="text-dim" />
                        Thumbnail
                    </label>
                    <p class="text-dim text-[10px]">
                        Generates preview thumbnails from video frames
                    </p>
                </div>
                <input
                    v-model.number="thumbnailWorkers"
                    @change="thumbnailWorkers = clamp(thumbnailWorkers)"
                    type="number"
                    min="1"
                    max="10"
                    class="border-border bg-surface w-16 rounded-lg border px-2 py-1.5 text-center
                        text-xs text-white focus:border-white/20 focus:outline-none"
                />
            </div>

            <!-- Sprites Workers -->
            <div class="flex items-center justify-between">
                <div>
                    <label class="flex items-center gap-1.5 text-xs font-medium text-white">
                        <Icon name="heroicons:squares-2x2" size="13" class="text-dim" />
                        Sprites
                    </label>
                    <p class="text-dim text-[10px]">
                        Builds sprite sheets and VTT files for seek preview
                    </p>
                </div>
                <input
                    v-model.number="spritesWorkers"
                    @change="spritesWorkers = clamp(spritesWorkers)"
                    type="number"
                    min="1"
                    max="10"
                    class="border-border bg-surface w-16 rounded-lg border px-2 py-1.5 text-center
                        text-xs text-white focus:border-white/20 focus:outline-none"
                />
            </div>

            <!-- Apply Button -->
            <div class="border-border flex items-center justify-between border-t pt-4">
                <span class="text-dim text-[10px]">Range: 1-10 workers per pool</span>
                <button
                    @click="applyConfig"
                    :disabled="saving"
                    class="bg-lava hover:bg-lava/90 rounded-lg px-4 py-1.5 text-xs font-medium
                        text-white transition-colors disabled:opacity-50"
                >
                    {{ saving ? 'Applying...' : 'Apply' }}
                </button>
            </div>
        </div>
    </div>
</template>
