<script setup lang="ts">
// Confirmation of a permanent deletion (one video or a selection): files are deleted from disk.
import type { TrashSelectionSummary } from '~/utils/trashFilters';

const props = defineProps<{
    visible: boolean;
    summary: TrashSelectionSummary;
    /** Title of the video when a single one is deleted */
    title?: string;
    loading: boolean;
}>();

const emit = defineEmits<{
    close: [];
    confirm: [];
}>();

const { formatSize } = useFormatter();
const understood = ref(false);

watch(
    () => props.visible,
    (v) => {
        if (v) understood.value = false;
    },
);

const label = computed(() => {
    const n = props.summary.count;
    return props.title && n === 1
        ? `"${props.title}"`
        : `${n.toLocaleString()} video${n === 1 ? '' : 's'}`;
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="!loading && emit('close')"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <div class="mb-3 flex items-center gap-2">
                    <Icon name="heroicons:exclamation-triangle" size="18" class="text-red-500" />
                    <h3 class="text-sm font-semibold text-white">Delete permanently</h3>
                </div>
                <p class="text-dim mb-3 text-xs">
                    Permanently delete <span class="text-white">{{ label }}</span
                    >, totalling
                    <span class="font-semibold text-white">{{ formatSize(summary.size) }}</span
                    ><span v-if="summary.unknownSize">
                        (plus {{ summary.unknownSize }} of unknown size)</span
                    >?
                </p>
                <div
                    class="mb-3 rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs
                        text-red-300"
                >
                    The video files are deleted from disk and cannot be recovered, along with their
                    scenes and generated files.
                </div>
                <p class="text-dim mb-4 text-[11px]">
                    Files on read-only storage paths are kept on disk<span
                        v-if="summary.keptOnDisk"
                    >
                        ({{ summary.keptOnDisk }} of the selected videos)</span
                    >; only their scene is removed.
                </p>
                <label class="mb-4 flex cursor-pointer items-start gap-2 text-xs text-white">
                    <input
                        v-model="understood"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-3.5 w-3.5 shrink-0"
                    />
                    I understand the files are deleted from disk for good
                </label>
                <div class="flex justify-end gap-2">
                    <button
                        :disabled="loading"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading || !understood"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="emit('confirm')"
                    >
                        {{
                            loading
                                ? 'Deleting...'
                                : `Delete ${summary.count.toLocaleString()} permanently`
                        }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
