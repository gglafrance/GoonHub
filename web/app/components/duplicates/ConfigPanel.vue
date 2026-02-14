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

        <div class="mb-3 grid grid-cols-1 gap-x-4 gap-y-1 sm:grid-cols-2">
            <!-- Audio thresholds -->
            <h4
                class="text-dim col-span-full mt-1 mb-2 text-[10px] font-medium tracking-wider
                    uppercase"
            >
                Audio
            </h4>

            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Density</label
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
                <p class="mt-1 text-[10px] text-white/30">
                    Ratio of matching hash positions within a matched segment. Higher = stricter,
                    requires more continuous overlap. Lower = catches re-encoded or partially
                    trimmed duplicates but may false-positive.
                </p>
            </div>
            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Min Hashes</label
                >
                <input
                    v-model.number="localConfig.audio_min_hashes"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Minimum number of matching hash positions required to accept a match. Higher =
                    ignores short or weak overlaps. Lower = detects shorter clips but risks false
                    positives.
                </p>
            </div>
            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Max Hash Freq</label
                >
                <input
                    v-model.number="localConfig.audio_max_hash_occurrences"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Hashes appearing in more than this many scenes are discarded as too common (e.g.
                    silence, intro music). Higher = keeps more hashes but may slow matching. Lower =
                    filters aggressively, improving speed and precision.
                </p>
            </div>
            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Min Span</label
                >
                <input
                    v-model.number="localConfig.audio_min_span"
                    type="number"
                    min="0"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Minimum duration of a match in hash positions (~8 positions/sec). Higher =
                    requires longer continuous overlap, rejecting short coincidental matches. Set to
                    0 to disable.
                </p>
            </div>

            <!-- Visual thresholds -->
            <h4
                class="text-dim col-span-full mt-2 mb-2 text-[10px] font-medium tracking-wider
                    uppercase"
            >
                Visual
            </h4>

            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Hamming Max</label
                >
                <input
                    v-model.number="localConfig.visual_hamming_max"
                    type="number"
                    min="0"
                    max="32"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Maximum bit-difference between two frame hashes to consider them matching
                    (0-32). Higher = tolerates re-encodes, resolution changes, and compression
                    artifacts. Lower = only near-identical frames match, fewer false positives.
                </p>
            </div>
            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Min Frames</label
                >
                <input
                    v-model.number="localConfig.visual_min_frames"
                    type="number"
                    min="1"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Minimum number of visually matching frames to accept a match. Higher = requires
                    more frames to align, reducing false positives. Lower = detects shorter clips
                    but may trigger on visually similar (non-duplicate) content.
                </p>
            </div>
            <div class="mb-3">
                <label class="text-dim mb-1 block text-[10px] tracking-wider uppercase"
                    >Min Span</label
                >
                <input
                    v-model.number="localConfig.visual_min_span"
                    type="number"
                    min="0"
                    class="border-border bg-panel w-full rounded-md border px-2.5 py-1.5 text-xs
                        text-white"
                />
                <p class="mt-1 text-[10px] text-white/30">
                    Minimum temporal span (in frames) that matched frames must cover. Prevents
                    clustered frame matches from a single moment being accepted as a full duplicate.
                    Set to 0 to disable.
                </p>
            </div>

            <!-- Shared -->
            <h4
                class="text-dim col-span-full mt-2 mb-2 text-[10px] font-medium tracking-wider
                    uppercase"
            >
                Alignment
            </h4>

            <div class="mb-3">
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
                <p class="mt-1 text-[10px] text-white/30">
                    Offset tolerance when aligning matches between two scenes. Controls bin width in
                    the alignment algorithm. Higher = tolerates slight timing differences (variable
                    frame rates, minor edits). Lower = requires precise frame-to-frame alignment.
                </p>
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
