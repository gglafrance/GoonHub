<script setup lang="ts">
import type { MarkerLabelMergeResult, MarkerLabelWithUsage } from '~/types/markerLabel';
import type ManageList from '~/components/settings/marker-labels/ManageList.vue';

// Settings > Marker Labels: the server-wide marker labels (marker_labels:manage). Create,
// rename (409 offers a merge), recolor, describe, merge, delete, and the default tags of each
// label (the same for every user).
const { fetchMarkerLabels, updateMarkerLabel } = useApiMarkerLabels();
const { message, error, clearMessages } = useSettingsMessage();

const labels = ref<MarkerLabelWithUsage[]>([]);
const loading = ref(false);
const list = ref<InstanceType<typeof ManageList> | null>(null);
const merge = ref<{ candidateIds: number[]; targetId: number | null } | null>(null);
const toDelete = ref<MarkerLabelWithUsage[] | null>(null);

onMounted(async () => {
    loading.value = true;
    await load();
    loading.value = false;
});

async function load() {
    try {
        labels.value = await fetchMarkerLabels();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load marker labels';
    }
}

function patch(id: number, fields: Partial<MarkerLabelWithUsage>) {
    labels.value = labels.value.map((l) => (l.id === id ? { ...l, ...fields } : l));
}

function onCreated(label: MarkerLabelWithUsage) {
    clearMessages();
    labels.value = [...labels.value, label].sort((a, b) =>
        a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
    );
    message.value = `Label "${label.name}" created`;
}

function onError(msg: string) {
    clearMessages();
    error.value = msg;
}

async function onColor(label: MarkerLabelWithUsage, color: string) {
    clearMessages();
    const previous = label.color;
    patch(label.id, { color });
    try {
        const saved = await updateMarkerLabel(label.id, { color });
        patch(label.id, { color: saved.color });
    } catch (e: unknown) {
        patch(label.id, { color: previous });
        error.value = e instanceof Error ? e.message : 'Failed to change the label color';
    }
}

function onRenamed(label: MarkerLabelWithUsage, name: string) {
    clearMessages();
    const previous = label.name;
    patch(label.id, { name });
    message.value = `Label "${previous}" renamed to "${name}"`;
}

function openMerge(candidateIds: number[], targetId: number | null = null) {
    clearMessages();
    merge.value = { candidateIds, targetId };
}

function openDelete(ids: number[]) {
    clearMessages();
    toDelete.value = labels.value.filter((l) => ids.includes(l.id));
}

function onDeleted(ids: number[], cleared: number) {
    labels.value = labels.value.filter((l) => !ids.includes(l.id));
    list.value?.clearSelection();
    message.value = `${plural(ids.length, 'label')} deleted, ${plural(cleared, 'marker')} left without label`;
}

async function onMerged(res: MarkerLabelMergeResult) {
    merge.value = null;
    list.value?.clearSelection();
    message.value = `Merged ${plural(res.sources_deleted, 'label')} into "${res.target.name}": ${plural(res.markers_moved, 'marker')} moved`;
    await load();
}
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <SettingsMarkerLabelsCreateForm @created="onCreated" @error="onError" />

        <SettingsMarkerLabelsManageList
            ref="list"
            :labels="labels"
            :loading="loading"
            @color="onColor"
            @renamed="onRenamed"
            @tags="(label, tags) => patch(label.id, { default_tags: tags })"
            @merge="openMerge"
            @merge-into="(source, target) => openMerge([source.id, target.id], target.id)"
            @delete="openDelete"
        />

        <SettingsMarkerLabelsDeleteModal
            :visible="toDelete !== null"
            :labels="toDelete ?? []"
            @close="toDelete = null"
            @deleted="onDeleted"
        />

        <SettingsMarkerLabelsMergeModal
            :visible="merge !== null"
            :labels="labels"
            :candidate-ids="merge?.candidateIds ?? []"
            :initial-target-id="merge?.targetId ?? null"
            @close="merge = null"
            @merged="onMerged"
        />
    </div>
</template>
