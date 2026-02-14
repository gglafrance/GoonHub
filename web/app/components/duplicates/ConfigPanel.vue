<script setup lang="ts">
import type { DuplicationConfig } from '~/types/duplicates';

const props = defineProps<{
    config: DuplicationConfig;
}>();

const emit = defineEmits<{
    save: [config: DuplicationConfig];
}>();

const localConfig = ref<DuplicationConfig>({ ...props.config });
const saving = ref(false);

watch(
    () => props.config,
    (val) => {
        localConfig.value = { ...val };
    },
);

const handleSave = async () => {
    saving.value = true;
    try {
        emit('save', { ...localConfig.value });
    } finally {
        saving.value = false;
    }
};
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-4">
        <h3 class="mb-4 text-sm font-medium text-white">Fingerprint Mode</h3>

        <div class="mb-6">
            <select
                v-model="localConfig.fingerprint_mode"
                class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                    text-white sm:w-64"
            >
                <option value="audio_only">Audio Only (default)</option>
                <option value="dual">Dual (audio + visual)</option>
            </select>
            <p class="text-dim mt-1.5 text-[10px]">
                Audio Only: videos with audio get audio fingerprints, silent videos get visual
                fingerprints. Dual: videos with audio get both fingerprint types, enabling
                cross-type matching.
            </p>
        </div>

        <h3 class="mb-4 text-sm font-medium text-white">Detection Thresholds</h3>

        <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Audio Density</label
                >
                <input
                    v-model.number="localConfig.audio_density_threshold"
                    type="number"
                    step="0.05"
                    min="0"
                    max="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Audio Min Hashes</label
                >
                <input
                    v-model.number="localConfig.audio_min_hashes"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Audio Max Hash Freq</label
                >
                <input
                    v-model.number="localConfig.audio_max_hash_occurrences"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Audio Min Span</label
                >
                <input
                    v-model.number="localConfig.audio_min_span"
                    type="number"
                    min="0"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Visual Hamming Max</label
                >
                <input
                    v-model.number="localConfig.visual_hamming_max"
                    type="number"
                    min="0"
                    max="32"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Visual Min Frames</label
                >
                <input
                    v-model.number="localConfig.visual_min_frames"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Visual Min Span</label
                >
                <input
                    v-model.number="localConfig.visual_min_span"
                    type="number"
                    min="0"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
            <div>
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Delta Tolerance</label
                >
                <input
                    v-model.number="localConfig.delta_tolerance"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
            </div>
        </div>

        <div class="mt-4 flex justify-end">
            <button
                :disabled="saving"
                class="bg-lava hover:bg-lava/80 rounded-md px-4 py-1.5 text-[11px] font-medium
                    text-white transition-all disabled:opacity-50"
                @click="handleSave"
            >
                {{ saving ? 'Saving...' : 'Save Config' }}
            </button>
        </div>
    </div>
</template>
