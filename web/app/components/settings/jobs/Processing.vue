<script setup lang="ts">
import type { ProcessingConfig } from '~/types/jobs';

const { fetchProcessingConfig, updateProcessingConfig } = useApi();

const loading = ref(true);
const saving = ref(false);
const error = ref('');
const message = ref('');

const maxFrameDimensionSm = ref(320);
const maxFrameDimensionLg = ref(1280);
const frameQualitySm = ref(85);
const frameQualityLg = ref(85);
const frameQualitySprites = ref(75);
const spritesConcurrency = ref(0);

const dimensionOptionsSm = [160, 240, 320, 480];
const dimensionOptionsLg = [640, 720, 960, 1280, 1920];

const loadConfig = async () => {
    loading.value = true;
    error.value = '';
    try {
        const config: ProcessingConfig = await fetchProcessingConfig();
        maxFrameDimensionSm.value = config.max_frame_dimension_sm;
        maxFrameDimensionLg.value = config.max_frame_dimension_lg;
        frameQualitySm.value = config.frame_quality_sm;
        frameQualityLg.value = config.frame_quality_lg;
        frameQualitySprites.value = config.frame_quality_sprites;
        spritesConcurrency.value = config.sprites_concurrency;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load processing config';
    } finally {
        loading.value = false;
    }
};

const applyConfig = async () => {
    saving.value = true;
    error.value = '';
    message.value = '';
    try {
        await updateProcessingConfig({
            max_frame_dimension_sm: maxFrameDimensionSm.value,
            max_frame_dimension_lg: maxFrameDimensionLg.value,
            frame_quality_sm: frameQualitySm.value,
            frame_quality_lg: frameQualityLg.value,
            frame_quality_sprites: frameQualitySprites.value,
            sprites_concurrency: spritesConcurrency.value,
        });
        message.value = 'Processing configuration updated';
        setTimeout(() => {
            message.value = '';
        }, 3000);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update processing config';
    } finally {
        saving.value = false;
    }
};

onMounted(() => {
    loadConfig();
});
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4">
            <h3 class="text-sm font-semibold text-white">Thumbnail & Quality Settings</h3>
            <p class="text-dim mt-1 text-[11px]">
                Configure resolution and quality for generated thumbnails and sprite sheets. Changes
                apply to newly processed videos only.
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

        <div v-else class="space-y-5">
            <!-- Resolution Section -->
            <div class="space-y-3">
                <h4 class="text-[11px] font-medium tracking-wider text-white/60 uppercase">
                    Max Frame Dimension
                </h4>

                <!-- Small Thumbnail Resolution -->
                <div class="flex items-center justify-between">
                    <div>
                        <label class="text-xs font-medium text-white">Small Thumbnail</label>
                        <p class="text-dim text-[10px]">Longest side in pixels for grid previews</p>
                    </div>
                    <select
                        v-model.number="maxFrameDimensionSm"
                        class="border-border bg-surface rounded-lg border px-2 py-1.5 text-xs
                            text-white focus:border-white/20 focus:outline-none"
                    >
                        <option v-for="dim in dimensionOptionsSm" :key="dim" :value="dim">
                            {{ dim }}px
                        </option>
                    </select>
                </div>

                <!-- Large Thumbnail Resolution -->
                <div class="flex items-center justify-between">
                    <div>
                        <label class="text-xs font-medium text-white">Large Thumbnail</label>
                        <p class="text-dim text-[10px]">
                            Longest side in pixels for detail/player view
                        </p>
                    </div>
                    <select
                        v-model.number="maxFrameDimensionLg"
                        class="border-border bg-surface rounded-lg border px-2 py-1.5 text-xs
                            text-white focus:border-white/20 focus:outline-none"
                    >
                        <option v-for="dim in dimensionOptionsLg" :key="dim" :value="dim">
                            {{ dim }}px
                        </option>
                    </select>
                </div>
            </div>

            <!-- Quality Section -->
            <div class="border-border space-y-3 border-t pt-5">
                <h4 class="text-[11px] font-medium tracking-wider text-white/60 uppercase">
                    WebP Quality (1-100)
                </h4>

                <!-- Small Thumbnail Quality -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <div>
                            <label class="text-xs font-medium text-white"
                                >Small Thumbnail Quality</label
                            >
                            <p class="text-dim text-[10px]">Lower values reduce file size</p>
                        </div>
                        <span class="font-mono text-xs text-white/80">{{ frameQualitySm }}</span>
                    </div>
                    <input
                        v-model.number="frameQualitySm"
                        type="range"
                        min="1"
                        max="100"
                        class="slider w-full"
                    />
                </div>

                <!-- Large Thumbnail Quality -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <div>
                            <label class="text-xs font-medium text-white"
                                >Large Thumbnail Quality</label
                            >
                            <p class="text-dim text-[10px]">
                                Used for detail view, higher is better
                            </p>
                        </div>
                        <span class="font-mono text-xs text-white/80">{{ frameQualityLg }}</span>
                    </div>
                    <input
                        v-model.number="frameQualityLg"
                        type="range"
                        min="1"
                        max="100"
                        class="slider w-full"
                    />
                </div>

                <!-- Sprites Quality -->
                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <div>
                            <label class="text-xs font-medium text-white"
                                >Sprite Sheet Quality</label
                            >
                            <p class="text-dim text-[10px]">
                                Seek preview sprites, lower saves disk space
                            </p>
                        </div>
                        <span class="font-mono text-xs text-white/80">{{
                            frameQualitySprites
                        }}</span>
                    </div>
                    <input
                        v-model.number="frameQualitySprites"
                        type="range"
                        min="1"
                        max="100"
                        class="slider w-full"
                    />
                </div>
            </div>

            <!-- Concurrency Section -->
            <div class="border-border space-y-3 border-t pt-5">
                <h4 class="text-[11px] font-medium tracking-wider text-white/60 uppercase">
                    Performance
                </h4>

                <div class="space-y-1.5">
                    <div class="flex items-center justify-between">
                        <div>
                            <label class="text-xs font-medium text-white"
                                >Sprites Concurrency</label
                            >
                            <p class="text-dim text-[10px]">
                                Parallel ffmpeg processes for frame extraction (0 = auto, uses CPU
                                count)
                            </p>
                        </div>
                        <input
                            v-model.number="spritesConcurrency"
                            type="number"
                            min="0"
                            max="64"
                            class="border-border bg-surface w-16 rounded-lg border px-2 py-1.5
                                text-center text-xs text-white focus:border-white/20
                                focus:outline-none"
                        />
                    </div>
                </div>
            </div>

            <!-- Apply Button -->
            <div class="border-border flex items-center justify-between border-t pt-4">
                <span class="text-dim text-[10px]">Applied to newly processed videos</span>
                <button
                    :disabled="saving"
                    class="bg-lava hover:bg-lava/90 rounded-lg px-4 py-1.5 text-xs font-medium
                        text-white transition-colors disabled:opacity-50"
                    @click="applyConfig"
                >
                    {{ saving ? 'Applying...' : 'Apply' }}
                </button>
            </div>
        </div>
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
    cursor: pointer;
    border: 2px solid rgba(0, 0, 0, 0.3);
}

.slider::-moz-range-thumb {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: #ff4d4d;
    cursor: pointer;
    border: 2px solid rgba(0, 0, 0, 0.3);
}
</style>
