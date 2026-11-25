<script setup lang="ts">
import type { MarkerLabelWithUsage } from '~/types/markerLabel';

// Deletes one or several marker labels. Their markers (of every user) are kept, without label
// and without the tags they had from it.
const props = defineProps<{
    visible: boolean;
    labels: MarkerLabelWithUsage[];
}>();

const emit = defineEmits<{
    close: [];
    deleted: [ids: number[], markersCleared: number];
}>();

const { deleteMarkerLabels } = useApiMarkerLabels();

const running = ref(false);
const error = ref('');

const markers = computed(() => props.labels.reduce((sum, l) => sum + l.marker_count, 0));

watch(
    () => props.visible,
    (v) => {
        if (v) error.value = '';
    },
);

async function confirm() {
    running.value = true;
    error.value = '';
    try {
        const ids = props.labels.map((l) => l.id);
        const res = await deleteMarkerLabels(ids);
        emit('deleted', ids, res.markers_cleared);
        emit('close');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete the labels';
    } finally {
        running.value = false;
    }
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                backdrop-blur-sm"
            @click.self="!running && emit('close')"
        >
            <div class="glass-panel w-full max-w-lg p-5">
                <h3 class="mb-3 text-sm font-semibold text-white">
                    Delete {{ plural(labels.length, 'label') }}
                </h3>
                <p class="text-dim mb-3 text-xs">
                    <span class="text-white">{{ plural(markers, 'marker') }}</span>
                    (of every user) will have no label. The markers themselves are kept. This cannot
                    be undone.
                </p>
                <div
                    class="border-border bg-void/60 mb-4 max-h-48 overflow-y-auto rounded-lg border
                        px-3 py-2"
                >
                    <div
                        v-for="label in labels"
                        :key="label.id"
                        class="flex items-center justify-between py-0.5 text-[11px]"
                    >
                        <span class="truncate text-white">{{ label.name }}</span>
                        <span class="text-dim ml-3 shrink-0">{{
                            plural(label.marker_count, 'marker')
                        }}</span>
                    </div>
                </div>

                <p v-if="error" class="text-lava mb-3 text-[11px]">{{ error }}</p>

                <div class="flex items-center justify-end gap-2">
                    <button
                        class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                            transition-colors hover:text-white disabled:opacity-40"
                        :disabled="running"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        class="flex items-center gap-1.5 rounded-lg bg-red-600 px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors hover:bg-red-500
                            disabled:opacity-40"
                        :disabled="running || labels.length === 0"
                        @click="confirm"
                    >
                        <Icon v-if="running" name="svg-spinners:ring-resize" size="12" />
                        <Icon v-else name="heroicons:trash" size="12" />
                        Delete {{ plural(labels.length, 'label') }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
