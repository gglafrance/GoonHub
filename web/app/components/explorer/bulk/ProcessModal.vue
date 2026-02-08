<script setup lang="ts">
import type { BulkJobResponse } from '~/types/jobs';

const props = defineProps<{
    visible: boolean;
    sceneIds?: number[];
    selectionCount?: number;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { triggerBulkPhase } = useApiJobs();

const resolvedSceneIds = computed(() => props.sceneIds ?? explorerStore.getSelectedSceneIDs());
const resolvedSelectionCount = computed(() => props.selectionCount ?? explorerStore.selectionCount);

const loading = ref(false);
const error = ref<string | null>(null);
const mode = ref<'missing' | 'all'>('all');

const phases = ref({
    metadata: false,
    thumbnail: false,
    sprites: false,
    animated_thumbnails: false,
});

const results = ref<Record<string, BulkJobResponse | null>>({
    metadata: null,
    thumbnail: null,
    sprites: null,
    animated_thumbnails: null,
});

const hasResults = computed(() => Object.values(results.value).some((r) => r !== null));
const selectedPhases = computed(() =>
    (Object.entries(phases.value) as [string, boolean][]).filter(([, v]) => v).map(([k]) => k),
);
const canSubmit = computed(() => selectedPhases.value.length > 0 && !loading.value);

const totalSubmitted = computed(() =>
    Object.values(results.value).reduce((sum, r) => sum + (r?.submitted ?? 0), 0),
);
const totalSkipped = computed(() =>
    Object.values(results.value).reduce((sum, r) => sum + (r?.skipped ?? 0), 0),
);
const totalErrors = computed(() =>
    Object.values(results.value).reduce((sum, r) => sum + (r?.errors ?? 0), 0),
);

const phaseLabel = (phase: string) => {
    switch (phase) {
        case 'metadata':
            return 'Metadata';
        case 'thumbnail':
            return 'Thumbnails';
        case 'sprites':
            return 'Sprites';
        case 'animated_thumbnails':
            return 'Animated Thumbnails';
        default:
            return phase;
    }
};

const phaseIcon = (phase: string) => {
    switch (phase) {
        case 'metadata':
            return 'heroicons:document-text';
        case 'thumbnail':
            return 'heroicons:photo';
        case 'sprites':
            return 'heroicons:squares-2x2';
        case 'animated_thumbnails':
            return 'heroicons:film';
        default:
            return 'heroicons:cog-6-tooth';
    }
};

const handleSubmit = async () => {
    loading.value = true;
    error.value = null;
    results.value = { metadata: null, thumbnail: null, sprites: null, animated_thumbnails: null };

    const sceneIds = resolvedSceneIds.value;

    for (const phase of selectedPhases.value) {
        try {
            results.value[phase] = await triggerBulkPhase(phase, mode.value, undefined, sceneIds);
        } catch (err) {
            results.value[phase] = { message: '', submitted: 0, skipped: 0, errors: 1 };
            error.value =
                err instanceof Error ? err.message : `Failed to trigger ${phaseLabel(phase)}`;
        }
    }

    loading.value = false;
};

const handleClose = () => {
    if (hasResults.value) {
        emit('complete');
    }
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="border-border bg-panel w-full max-w-md rounded-xl border shadow-2xl">
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <div class="flex items-center gap-2">
                        <Icon name="heroicons:cpu-chip" size="16" class="text-lava" />
                        <h2 class="text-sm font-semibold text-white">Process Scenes</h2>
                    </div>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Content -->
                <div class="p-4">
                    <p class="text-dim mb-4 text-xs">
                        Trigger processing jobs for
                        <span class="font-medium text-white">{{ resolvedSelectionCount }}</span>
                        selected scene{{ resolvedSelectionCount === 1 ? '' : 's' }}.
                    </p>

                    <!-- Phase Checkboxes -->
                    <div class="mb-4 space-y-2">
                        <label
                            v-for="phase in [
                                'metadata',
                                'thumbnail',
                                'sprites',
                                'animated_thumbnails',
                            ] as const"
                            :key="phase"
                            class="bg-surface/30 border-border flex cursor-pointer items-center
                                gap-3 rounded-lg border px-3 py-2 transition-colors
                                hover:border-white/15"
                        >
                            <input
                                v-model="phases[phase]"
                                type="checkbox"
                                :disabled="loading"
                                class="accent-lava h-3.5 w-3.5 shrink-0 rounded"
                            />
                            <Icon :name="phaseIcon(phase)" size="14" class="text-dim" />
                            <span class="text-xs font-medium text-white">{{
                                phaseLabel(phase)
                            }}</span>
                        </label>
                    </div>

                    <!-- Mode Toggle -->
                    <div class="bg-surface/30 border-border mb-4 rounded-lg border p-3">
                        <span class="text-dim mb-2 block text-xs font-medium">Mode</span>
                        <div class="flex gap-2">
                            <button
                                :disabled="loading"
                                class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium
                                    transition-all"
                                :class="
                                    mode === 'all'
                                        ? 'bg-lava/15 border-lava/30 text-lava border'
                                        : 'border-border text-dim border hover:text-white'
                                "
                                @click="mode = 'all'"
                            >
                                All
                            </button>
                            <button
                                :disabled="loading"
                                class="flex-1 rounded-md px-3 py-1.5 text-xs font-medium
                                    transition-all"
                                :class="
                                    mode === 'missing'
                                        ? 'bg-lava/15 border-lava/30 text-lava border'
                                        : 'border-border text-dim border hover:text-white'
                                "
                                @click="mode = 'missing'"
                            >
                                Missing Only
                            </button>
                        </div>
                        <p class="text-dim mt-2 text-[11px]">
                            {{
                                mode === 'all'
                                    ? 'Force reprocess all selected scenes, even if already processed.'
                                    : 'Only process scenes that are missing data for the selected phases.'
                            }}
                        </p>
                    </div>

                    <!-- Results -->
                    <div
                        v-if="hasResults"
                        class="bg-surface/30 border-border mb-4 rounded-lg border p-3"
                    >
                        <span class="text-dim mb-2 block text-xs font-medium">Results</span>
                        <div class="space-y-1.5">
                            <div
                                v-for="phase in selectedPhases"
                                :key="phase"
                                class="flex items-center justify-between"
                            >
                                <span class="text-xs text-white">{{ phaseLabel(phase) }}</span>
                                <span v-if="results[phase]" class="text-dim text-xs">
                                    <span class="text-emerald-400">{{
                                        results[phase]!.submitted
                                    }}</span>
                                    submitted
                                    <template v-if="results[phase]!.skipped > 0">
                                        ,
                                        <span class="text-amber-400">{{
                                            results[phase]!.skipped
                                        }}</span>
                                        skipped
                                    </template>
                                    <template v-if="results[phase]!.errors > 0">
                                        ,
                                        <span class="text-red-400">{{
                                            results[phase]!.errors
                                        }}</span>
                                        errors
                                    </template>
                                </span>
                                <Icon
                                    v-else
                                    name="svg-spinners:90-ring-with-bg"
                                    size="12"
                                    class="text-dim"
                                />
                            </div>
                        </div>
                        <!-- Summary -->
                        <div class="border-border mt-2 border-t pt-2">
                            <div class="flex items-center justify-between">
                                <span class="text-dim text-xs font-medium">Total</span>
                                <span class="text-xs">
                                    <span class="text-emerald-400">{{ totalSubmitted }}</span>
                                    submitted
                                    <template v-if="totalSkipped > 0">
                                        ,
                                        <span class="text-amber-400">{{ totalSkipped }}</span>
                                        skipped
                                    </template>
                                    <template v-if="totalErrors > 0">
                                        ,
                                        <span class="text-red-400">{{ totalErrors }}</span>
                                        errors
                                    </template>
                                </span>
                            </div>
                        </div>
                    </div>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />
                </div>

                <!-- Footer -->
                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        :disabled="loading"
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all disabled:opacity-50"
                        @click="handleClose"
                    >
                        {{ hasResults ? 'Done' : 'Cancel' }}
                    </button>
                    <button
                        v-if="!hasResults"
                        :disabled="!canSubmit"
                        class="bg-lava hover:bg-lava/90 rounded-lg px-3 py-1.5 text-xs font-semibold
                            text-white transition-colors disabled:opacity-50"
                        @click="handleSubmit"
                    >
                        <template v-if="loading">
                            <Icon
                                name="svg-spinners:90-ring-with-bg"
                                size="14"
                                class="mr-1 inline"
                            />
                            Processing...
                        </template>
                        <template v-else> Process Scenes </template>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
