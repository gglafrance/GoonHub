<script setup lang="ts">
import type { MarkerLabelMergeResult, MarkerLabelWithUsage } from '~/types/markerLabel';

// Merge of several marker labels into one: pick the label to keep, see what the merge changes
// (dry run), confirm. The other labels are deleted, so it cannot be undone.
const props = defineProps<{
    visible: boolean;
    labels: MarkerLabelWithUsage[];
    candidateIds: number[];
    initialTargetId: number | null;
}>();

const emit = defineEmits<{
    close: [];
    merged: [result: MarkerLabelMergeResult];
}>();

const { mergeMarkerLabels } = useApiMarkerLabels();

const targetId = ref<number | null>(null);
const preview = ref<MarkerLabelMergeResult | null>(null);
const previewLoading = ref(false);
const merging = ref(false);
const error = ref('');
let previewToken = 0;

const candidates = computed(() =>
    props.candidateIds
        .map((id) => props.labels.find((l) => l.id === id))
        .filter((l): l is MarkerLabelWithUsage => !!l),
);
const target = computed(() => candidates.value.find((l) => l.id === targetId.value));
const sourceIds = computed(() => mergeSourceIds(props.candidateIds, targetId.value));
const sources = computed(() => candidates.value.filter((l) => sourceIds.value.includes(l.id)));

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return;
        error.value = '';
        preview.value = null;
        targetId.value =
            props.initialTargetId ?? defaultMarkerLabelMergeTarget(candidates.value)?.id ?? null;
    },
    { immediate: true },
);

watch([targetId, () => props.visible], loadPreview);

async function loadPreview() {
    const token = ++previewToken;
    preview.value = null;
    error.value = '';
    if (!props.visible || targetId.value === null || sourceIds.value.length === 0) return;
    previewLoading.value = true;
    try {
        const res = await mergeMarkerLabels(targetId.value, sourceIds.value, true);
        if (token === previewToken) preview.value = res;
    } catch (e: unknown) {
        if (token === previewToken) {
            error.value = e instanceof Error ? e.message : 'Failed to preview the merge';
        }
    } finally {
        if (token === previewToken) previewLoading.value = false;
    }
}

async function confirm() {
    if (targetId.value === null || sourceIds.value.length === 0) return;
    merging.value = true;
    error.value = '';
    try {
        emit('merged', await mergeMarkerLabels(targetId.value, sourceIds.value));
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to merge the labels';
    } finally {
        merging.value = false;
    }
}

function close() {
    if (!merging.value) emit('close');
}

function onKeydown(e: KeyboardEvent) {
    if (props.visible && e.key === 'Escape') close();
}

onMounted(() => window.addEventListener('keydown', onKeydown));
onUnmounted(() => window.removeEventListener('keydown', onKeydown));
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="close"
        >
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-1 text-sm font-semibold text-white">Merge Marker Labels</h3>
                <p class="text-dim mb-4 text-xs">
                    Every marker of a merged label (of every user) gets the kept label, which also
                    gets their default tags. The merged labels are deleted.
                </p>

                <p class="text-dim mb-1.5 text-[11px] font-medium tracking-wider uppercase">
                    Keep this label
                </p>
                <div class="space-y-1">
                    <label
                        v-for="label in candidates"
                        :key="label.id"
                        class="border-border flex cursor-pointer items-center gap-2.5 rounded-lg
                            border px-3 py-1.5 transition-colors hover:bg-white/2"
                        :class="targetId === label.id ? 'border-lava/40 bg-lava/5' : ''"
                    >
                        <input
                            v-model="targetId"
                            type="radio"
                            :value="label.id"
                            :disabled="merging"
                            class="accent-lava h-3.5 w-3.5"
                        />
                        <span
                            class="h-2.5 w-2.5 rounded-full"
                            :style="{ backgroundColor: label.color }"
                        />
                        <span class="flex-1 truncate text-xs whitespace-pre text-white">{{
                            label.name
                        }}</span>
                        <span class="text-dim text-[11px]">{{
                            plural(label.marker_count, 'marker')
                        }}</span>
                    </label>
                </div>

                <div
                    v-if="target && sources.length > 0"
                    class="bg-surface/50 border-border mt-4 space-y-2 rounded-lg border p-3 text-xs"
                >
                    <div v-if="previewLoading" class="text-dim flex items-center gap-2">
                        <LoadingSpinner /> Counting...
                    </div>
                    <ul v-else-if="preview" class="text-muted space-y-0.5">
                        <li>
                            <span class="text-white">{{
                                plural(preview.markers_moved, 'marker')
                            }}</span>
                            moved to "{{ target.name }}"
                        </li>
                        <li v-if="preview.default_tags_added > 0">
                            <span class="text-white">{{
                                plural(preview.default_tags_added, 'default tag')
                            }}</span>
                            added
                        </li>
                        <li v-if="preview.references_updated > 0">
                            <span class="text-white">{{
                                plural(preview.references_updated, 'saved filter or homepage')
                            }}</span>
                            updated
                        </li>
                        <li class="text-lava">
                            {{ plural(preview.sources_deleted, 'label') }} deleted. This cannot be
                            undone.
                        </li>
                    </ul>
                </div>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mt-4 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <div class="mt-5 flex justify-end gap-2">
                    <button
                        :disabled="merging"
                        class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                            hover:text-white disabled:opacity-40"
                        @click="close"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="merging || !preview || sources.length === 0"
                        class="rounded-lg bg-red-600 px-4 py-1.5 text-xs font-semibold text-white
                            transition-all hover:bg-red-500 disabled:cursor-not-allowed
                            disabled:opacity-40"
                        @click="confirm"
                    >
                        {{ merging ? 'Merging...' : `Merge ${plural(sources.length, 'label')}` }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
