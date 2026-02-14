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

const phaseOptions = [
    { key: 'metadata' as const, label: 'Metadata', icon: 'heroicons:document-text' },
    { key: 'thumbnail' as const, label: 'Thumbnails', icon: 'heroicons:photo' },
    { key: 'sprites' as const, label: 'Sprites', icon: 'heroicons:squares-2x2' },
    { key: 'animated_thumbnails' as const, label: 'Animated Thumbnails', icon: 'heroicons:film' },
    { key: 'fingerprint' as const, label: 'Fingerprint', icon: 'heroicons:finger-print' },
] as const;

const modeOptions = [
    {
        key: 'all' as const,
        label: 'All',
        icon: 'heroicons:arrow-path',
        desc: 'Force reprocess all selected scenes',
    },
    {
        key: 'missing' as const,
        label: 'Missing Only',
        icon: 'heroicons:funnel',
        desc: 'Only process scenes missing data',
    },
] as const;

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') handleClose();
}

const handleSubmit = async () => {
    loading.value = true;
    error.value = null;
    results.value = { metadata: null, thumbnail: null, sprites: null, animated_thumbnails: null };

    const sceneIds = resolvedSceneIds.value;

    for (const phase of selectedPhases.value) {
        try {
            results.value[phase] = await triggerBulkPhase(phase, mode.value, undefined, sceneIds);
        } catch (err) {
            const phaseLabel = phaseOptions.find((p) => p.key === phase)?.label ?? phase;
            results.value[phase] = { message: '', submitted: 0, skipped: 0, errors: 1 };
            error.value = err instanceof Error ? err.message : `Failed to trigger ${phaseLabel}`;
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
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="handleClose"
                @keydown="onKeydown"
            >
                <Transition
                    enter-active-class="transition duration-200 ease-out"
                    enter-from-class="scale-95 opacity-0"
                    enter-to-class="scale-100 opacity-100"
                    leave-active-class="transition duration-150 ease-in"
                    leave-from-class="scale-100 opacity-100"
                    leave-to-class="scale-95 opacity-0"
                    appear
                >
                    <div
                        class="border-border bg-panel flex w-full max-w-md flex-col rounded-xl
                            border shadow-2xl"
                    >
                        <!-- Header -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-b
                                px-4 py-3"
                        >
                            <div class="flex items-center gap-2.5">
                                <div
                                    class="bg-lava/10 flex h-6 w-6 items-center justify-center
                                        rounded-lg"
                                >
                                    <Icon name="heroicons:cpu-chip" size="13" class="text-lava" />
                                </div>
                                <div>
                                    <h2 class="text-sm font-semibold text-white">Process Scenes</h2>
                                    <p class="text-dim text-[10px] leading-tight">
                                        {{ resolvedSelectionCount }} scenes selected
                                    </p>
                                </div>
                            </div>
                            <button
                                class="text-dim flex items-center justify-center rounded-lg p-1.5
                                    transition-colors hover:bg-white/5 hover:text-white"
                                @click="handleClose"
                            >
                                <Icon name="heroicons:x-mark" size="16" />
                            </button>
                        </div>

                        <!-- Content -->
                        <div class="p-4">
                            <!-- Phase checkboxes -->
                            <div class="mb-4 space-y-1.5">
                                <button
                                    v-for="phase in phaseOptions"
                                    :key="phase.key"
                                    :disabled="loading"
                                    class="flex w-full items-center gap-3 rounded-lg px-3 py-2.5
                                        transition-all"
                                    :class="phases[phase.key] ? 'bg-lava/4' : 'hover:bg-white/3'"
                                    @click="phases[phase.key] = !phases[phase.key]"
                                >
                                    <div
                                        class="flex h-4 w-4 shrink-0 items-center justify-center
                                            rounded border transition-all"
                                        :class="
                                            phases[phase.key]
                                                ? 'border-lava bg-lava/20'
                                                : 'border-border'
                                        "
                                    >
                                        <Icon
                                            v-if="phases[phase.key]"
                                            name="heroicons:check"
                                            size="10"
                                            class="text-lava"
                                        />
                                    </div>
                                    <Icon
                                        :name="phase.icon"
                                        size="14"
                                        class="transition-colors"
                                        :class="phases[phase.key] ? 'text-lava' : 'text-dim'"
                                    />
                                    <span
                                        class="text-xs font-medium transition-colors"
                                        :class="phases[phase.key] ? 'text-white' : 'text-white/70'"
                                    >
                                        {{ phase.label }}
                                    </span>
                                </button>
                            </div>

                            <!-- Mode selector -->
                            <div class="mb-4">
                                <div class="bg-surface flex gap-0.5 rounded-lg p-0.5">
                                    <button
                                        v-for="m in modeOptions"
                                        :key="m.key"
                                        :disabled="loading"
                                        class="flex flex-1 items-center justify-center gap-1.5
                                            rounded-md py-1.5 text-[11px] font-medium
                                            transition-all"
                                        :class="
                                            mode === m.key
                                                ? 'bg-lava/15 text-lava shadow-sm'
                                                : 'text-dim hover:text-white'
                                        "
                                        @click="mode = m.key"
                                    >
                                        <Icon :name="m.icon" size="12" />
                                        {{ m.label }}
                                    </button>
                                </div>
                                <p class="text-dim mt-1.5 px-0.5 text-[10px]">
                                    {{ modeOptions.find((m) => m.key === mode)?.desc }}
                                </p>
                            </div>

                            <!-- Results -->
                            <div v-if="hasResults" class="mb-4">
                                <div class="border-border rounded-lg border">
                                    <div class="space-y-0">
                                        <div
                                            v-for="(phase, idx) in selectedPhases"
                                            :key="phase"
                                            class="flex items-center justify-between px-3 py-2"
                                            :class="
                                                idx < selectedPhases.length - 1
                                                    ? 'border-border border-b'
                                                    : ''
                                            "
                                        >
                                            <div class="flex items-center gap-2">
                                                <Icon
                                                    :name="
                                                        phaseOptions.find((p) => p.key === phase)
                                                            ?.icon ?? 'heroicons:cog-6-tooth'
                                                    "
                                                    size="12"
                                                    class="text-dim"
                                                />
                                                <span class="text-xs text-white">
                                                    {{
                                                        phaseOptions.find((p) => p.key === phase)
                                                            ?.label ?? phase
                                                    }}
                                                </span>
                                            </div>
                                            <span
                                                v-if="results[phase]"
                                                class="text-dim text-[11px]"
                                            >
                                                <span class="text-emerald-400">
                                                    {{ results[phase]!.submitted }}
                                                </span>
                                                sent
                                                <template v-if="results[phase]!.skipped > 0">
                                                    /
                                                    <span class="text-amber-400">
                                                        {{ results[phase]!.skipped }}
                                                    </span>
                                                    skip
                                                </template>
                                                <template v-if="results[phase]!.errors > 0">
                                                    /
                                                    <span class="text-red-400">
                                                        {{ results[phase]!.errors }}
                                                    </span>
                                                    err
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
                                    <div
                                        class="border-border bg-surface/30 flex items-center
                                            justify-between border-t px-3 py-2"
                                    >
                                        <span class="text-dim text-[11px] font-medium">Total</span>
                                        <span class="text-[11px]">
                                            <span class="text-emerald-400">
                                                {{ totalSubmitted }}
                                            </span>
                                            submitted
                                            <template v-if="totalSkipped > 0">
                                                ,
                                                <span class="text-amber-400">
                                                    {{ totalSkipped }}
                                                </span>
                                                skipped
                                            </template>
                                            <template v-if="totalErrors > 0">
                                                ,
                                                <span class="text-red-400">
                                                    {{ totalErrors }}
                                                </span>
                                                errors
                                            </template>
                                        </span>
                                    </div>
                                </div>
                            </div>

                            <!-- Error -->
                            <div v-if="error" class="mb-4">
                                <div
                                    class="border-lava/20 bg-lava/5 flex items-center gap-2
                                        rounded-lg border px-3 py-2"
                                >
                                    <Icon
                                        name="heroicons:exclamation-triangle"
                                        size="13"
                                        class="text-lava shrink-0"
                                    />
                                    <span class="text-[11px] text-red-300">{{ error }}</span>
                                </div>
                            </div>
                        </div>

                        <!-- Footer -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-t
                                px-4 py-3"
                        >
                            <span class="text-dim text-[11px]">
                                <template v-if="selectedPhases.length > 0">
                                    <span class="text-lava font-medium">
                                        {{ selectedPhases.length }}
                                    </span>
                                    phase{{ selectedPhases.length === 1 ? '' : 's' }}
                                </template>
                                <template v-else>No phases selected</template>
                            </span>
                            <div class="flex items-center gap-2">
                                <button
                                    :disabled="loading"
                                    class="border-border hover:border-border-hover rounded-lg border
                                        px-3 py-1.5 text-xs font-medium text-white transition-all
                                        disabled:opacity-50"
                                    @click="handleClose"
                                >
                                    {{ hasResults ? 'Done' : 'Cancel' }}
                                </button>
                                <button
                                    v-if="!hasResults"
                                    :disabled="!canSubmit"
                                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                                        font-semibold text-white transition-colors
                                        disabled:opacity-50"
                                    @click="handleSubmit"
                                >
                                    <span v-if="loading" class="flex items-center gap-1.5">
                                        <Icon name="svg-spinners:90-ring-with-bg" size="12" />
                                        Processing
                                    </span>
                                    <span v-else>Process</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </Transition>
    </Teleport>
</template>
