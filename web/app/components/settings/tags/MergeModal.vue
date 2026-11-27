<script setup lang="ts">
import type { Tag, TagMergeResult } from '~/types/tag';

// Merge of several tags into one: pick the tag to keep, see what the merge changes (dry run),
// confirm. The sources are deleted, so it cannot be undone.
const props = defineProps<{
    visible: boolean;
    tags: Tag[];
    candidateIds: number[];
    initialTargetId: number | null;
}>();

const emit = defineEmits<{
    close: [];
    merged: [result: TagMergeResult];
}>();

const { mergeTags } = useApiTags();

const targetId = ref<number | null>(null);
const preview = ref<TagMergeResult | null>(null);
const previewLoading = ref(false);
const merging = ref(false);
const error = ref('');
let previewToken = 0;

const candidates = computed(() =>
    props.candidateIds
        .map((id) => props.tags.find((t) => t.id === id))
        .filter((t): t is Tag => !!t),
);
const others = computed(() => props.tags.filter((t) => !props.candidateIds.includes(t.id)));
const target = computed(() => props.tags.find((t) => t.id === targetId.value));
const sourceIds = computed(() => mergeSourceIds(props.candidateIds, targetId.value));
const sources = computed(() => props.tags.filter((t) => sourceIds.value.includes(t.id)));

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return;
        error.value = '';
        preview.value = null;
        targetId.value =
            props.initialTargetId ??
            (candidates.value.length > 1
                ? (defaultMergeTarget(candidates.value)?.id ?? null)
                : null);
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
        const res: TagMergeResult = await mergeTags(targetId.value, sourceIds.value, true);
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
        const res: TagMergeResult = await mergeTags(targetId.value, sourceIds.value);
        emit('merged', res);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to merge tags';
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
                <h3 class="mb-1 text-sm font-semibold text-white">Merge Tags</h3>
                <p class="text-dim mb-4 text-xs">
                    Every scene, marker and playlist tagged with a merged tag gets the kept tag
                    instead, and the merged tags are deleted.
                </p>

                <SettingsTagsMergeTargetPicker
                    v-model="targetId"
                    :candidates="candidates"
                    :others="others"
                    :disabled="merging"
                />

                <div
                    v-if="target && sources.length > 0"
                    class="bg-surface/50 border-border mt-4 space-y-2 rounded-lg border p-3 text-xs"
                >
                    <p class="text-muted">
                        Merge
                        <span v-for="(s, i) in sources" :key="s.id"
                            ><span class="whitespace-pre text-white">"{{ s.name }}"</span
                            >{{ i < sources.length - 1 ? ', ' : '' }}</span
                        >
                        into <span class="whitespace-pre text-white">"{{ target.name }}"</span>.
                    </p>
                    <div v-if="previewLoading" class="text-dim flex items-center gap-2">
                        <LoadingSpinner /> Counting...
                    </div>
                    <ul v-else-if="preview" class="text-muted space-y-0.5">
                        <li>
                            <span class="text-white">{{
                                plural(preview.scenes_retagged, 'scene')
                            }}</span>
                            retagged
                        </li>
                        <li v-if="preview.markers_updated + preview.label_tags_updated > 0">
                            <span class="text-white">{{
                                plural(preview.markers_updated, 'marker')
                            }}</span>
                            and
                            <span class="text-white">{{
                                plural(preview.label_tags_updated, 'label default')
                            }}</span>
                            updated
                        </li>
                        <li v-if="preview.playlists_updated > 0">
                            <span class="text-white">{{
                                plural(preview.playlists_updated, 'playlist')
                            }}</span>
                            updated
                        </li>
                        <li v-if="preview.references_updated > 0">
                            <span class="text-white">{{
                                plural(preview.references_updated, 'saved filter or homepage')
                            }}</span>
                            updated
                        </li>
                        <li class="text-lava">
                            {{ plural(preview.sources_deleted, 'tag') }} deleted. This cannot be
                            undone.
                        </li>
                    </ul>
                </div>
                <p v-else-if="candidates.length === 1 && !target" class="text-dim mt-4 text-xs">
                    Search for the tag to merge "{{ candidates[0]?.name }}" into.
                </p>

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
                        {{ merging ? 'Merging...' : `Merge ${plural(sources.length, 'tag')}` }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
