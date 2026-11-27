<script setup lang="ts">
import type { HealthAction, HealthActionPreview } from '~/types/libraryHealth';

// Confirmation of a health page action. It asks the server how many jobs (or files) the
// action would touch and states it before anything runs: a bulk run once queued 30,900
// fingerprint jobs by surprise.
const props = defineProps<{
    action: HealthAction | null;
    /** Scenes of the category behind the action, to explain the ones that are skipped */
    categoryCount?: number;
}>();

const emit = defineEmits<{
    close: [];
    confirm: [action: HealthAction];
}>();

const LARGE_BATCH = 1000;

const { previewAction } = useApiLibraryHealth();

const loading = ref(false);
const error = ref('');
const preview = ref<HealthActionPreview | null>(null);

watch(
    () => props.action,
    async (action) => {
        preview.value = null;
        error.value = '';
        if (!action) return;
        loading.value = true;
        try {
            const result = await previewAction(action);
            if (props.action === action) preview.value = result;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to count';
        } finally {
            loading.value = false;
        }
    },
);

const isPhase = computed(() => props.action !== null && isHealthPhase(props.action));

const jobsText = computed(() => {
    if (!props.action || !isHealthPhase(props.action) || !preview.value) return '';
    return `${preview.value.count.toLocaleString()} ${healthPhaseLabel(props.action)} jobs`;
});

const title = computed(() => {
    if (!props.action) return '';
    if (isHealthPhase(props.action)) return `Queue ${healthPhaseLabel(props.action)} jobs?`;
    return props.action === 'fix_sizes' ? 'Fix file sizes?' : 'Check every file?';
});

const confirmLabel = computed(() => {
    const count = preview.value?.count ?? 0;
    if (isPhase.value) return `Queue ${count.toLocaleString()} jobs`;
    return props.action === 'fix_sizes' ? 'Fix sizes' : 'Start check';
});

const skipped = computed(() => {
    if (!preview.value || props.categoryCount === undefined) return 0;
    return Math.max(0, props.categoryCount - preview.value.count);
});

const canConfirm = computed(
    () => !loading.value && !!preview.value && preview.value.enabled && preview.value.count > 0,
);

const handleEscape = (e: KeyboardEvent) => {
    if (props.action && e.key === 'Escape') emit('close');
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
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
                v-if="action"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="emit('close')"
            >
                <div
                    class="border-border bg-panel mx-4 w-full max-w-sm rounded-xl border p-5
                        shadow-2xl"
                >
                    <div class="mb-4 flex items-start gap-3">
                        <div class="bg-lava/10 flex items-center justify-center rounded-full p-2">
                            <Icon
                                :name="
                                    isPhase ? 'heroicons:queue-list' : 'heroicons:magnifying-glass'
                                "
                                size="20"
                                class="text-lava"
                            />
                        </div>
                        <div class="min-w-0">
                            <h3 class="text-sm font-medium text-white">{{ title }}</h3>
                            <div class="text-dim mt-1 space-y-2 text-xs">
                                <p v-if="loading">Counting...</p>
                                <p v-else-if="error" class="text-lava">{{ error }}</p>
                                <template v-else-if="preview">
                                    <p v-if="!preview.enabled" class="text-amber-500">
                                        {{ preview.reason }}
                                    </p>
                                    <template v-else-if="isPhase">
                                        <p v-if="preview.count === 0">
                                            Nothing to queue: every scene already has it, has no
                                            metadata yet, or has a job queued.
                                        </p>
                                        <p v-else>
                                            This queues
                                            <span class="font-semibold text-white">{{
                                                jobsText
                                            }}</span
                                            >, one per scene that misses it.
                                        </p>
                                        <p v-if="skipped > 0">
                                            {{ skipped.toLocaleString() }} other scenes of this
                                            category are left out: they have no metadata yet or
                                            already have a job queued.
                                        </p>
                                    </template>
                                    <template v-else-if="action === 'fix_sizes'">
                                        <p>
                                            Re-reads from disk the size of
                                            <span class="font-semibold text-white">
                                                {{ preview.count.toLocaleString() }} scenes
                                            </span>
                                            stored with size 0, then updates the database and the
                                            search index. Files are only read.
                                        </p>
                                    </template>
                                    <template v-else>
                                        <p>
                                            Looks at
                                            <span class="font-semibold text-white">
                                                {{ preview.count.toLocaleString() }} files
                                            </span>
                                            on disk, in the background. On network storage this can
                                            take a while; you can cancel it. Files are only read,
                                            never modified.
                                        </p>
                                    </template>
                                    <p
                                        v-if="isPhase && preview.count >= LARGE_BATCH"
                                        class="text-amber-500"
                                    >
                                        This is a large batch: the workers stay busy until it is
                                        done.
                                    </p>
                                </template>
                            </div>
                        </div>
                    </div>
                    <div class="flex justify-end gap-2">
                        <button
                            class="border-border hover:bg-surface-hover rounded-lg border px-3
                                py-1.5 text-xs text-white transition-colors"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            :disabled="!canConfirm"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                                font-semibold text-white transition-colors
                                disabled:cursor-not-allowed disabled:opacity-40"
                            @click="action && emit('confirm', action)"
                        >
                            {{ confirmLabel }}
                        </button>
                    </div>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>
