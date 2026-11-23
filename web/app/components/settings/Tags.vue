<script setup lang="ts">
import type { Tag, TagDuplicateGroup, TagMergeResult } from '~/types/tag';
import type { TagSort } from '~/types/settings';
import type ManageList from '~/components/settings/tags/ManageList.vue';

const { fetchTags, deleteTag, updateTagColor, fetchDuplicateTags } = useApiTags();
const { message, error, clearMessages } = useSettingsMessage();
const settingsStore = useSettingsStore();

const tags = ref<Tag[]>([]);
const duplicates = ref<TagDuplicateGroup[]>([]);
const loading = ref(false);
const list = ref<InstanceType<typeof ManageList> | null>(null);

const merge = ref<{ candidateIds: number[]; targetId: number | null } | null>(null);
// tags selected for a bulk delete
const deleteMany = ref<Tag[] | null>(null);

const selectedTagSort = computed({
    get: () => (settingsStore.draft?.default_tag_sort ?? 'az') as TagSort,
    set: (v: TagSort) => {
        if (settingsStore.draft) settingsStore.draft.default_tag_sort = v;
    },
});

const tagSortOptions: { value: TagSort; label: string }[] = [
    { value: 'az', label: 'A-Z' },
    { value: 'za', label: 'Z-A' },
    { value: 'most', label: 'Most used' },
    { value: 'least', label: 'Least used' },
];

onMounted(async () => {
    loading.value = true;
    await loadTags();
    loading.value = false;
});

async function loadTags() {
    try {
        const [tagsRes, dupRes] = await Promise.all([fetchTags(), fetchDuplicateTags()]);
        tags.value = tagsRes.data || [];
        duplicates.value = dupRes.data || [];
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load tags';
    }
}

async function refreshDuplicates() {
    try {
        duplicates.value = (await fetchDuplicateTags()).data || [];
    } catch {
        // the banner is a hint: keep the previous groups
    }
}

function onCreated(tag: Tag) {
    clearMessages();
    tags.value = [...tags.value, tag].sort((a, b) => a.name.localeCompare(b.name));
    message.value = `Tag "${tag.name}" created`;
    refreshDuplicates();
}

function onCreateError(msg: string) {
    clearMessages();
    error.value = msg;
}

async function handleDelete(tag: Tag) {
    clearMessages();
    try {
        await deleteTag(tag.id);
        tags.value = tags.value.filter((t) => t.id !== tag.id);
        message.value = `Tag "${tag.name}" deleted`;
        refreshDuplicates();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete tag';
    }
}

async function handleColor(tag: Tag, color: string) {
    clearMessages();
    const previous = tag.color;
    tags.value = tags.value.map((t) => (t.id === tag.id ? { ...t, color } : t));
    try {
        const saved: Tag = await updateTagColor(tag.id, color);
        tags.value = tags.value.map((t) => (t.id === tag.id ? { ...t, color: saved.color } : t));
    } catch (e: unknown) {
        tags.value = tags.value.map((t) => (t.id === tag.id ? { ...t, color: previous } : t));
        error.value = e instanceof Error ? e.message : 'Failed to change the tag color';
    }
}

function onRenamed(tag: Tag) {
    clearMessages();
    const previous = tags.value.find((t) => t.id === tag.id);
    tags.value = tags.value.map((t) => (t.id === tag.id ? { ...t, ...tag } : t));
    message.value = previous
        ? `Tag "${previous.name}" renamed to "${tag.name}"`
        : `Tag renamed to "${tag.name}"`;
    refreshDuplicates();
}

function openMerge(candidateIds: number[], targetId: number | null = null) {
    clearMessages();
    merge.value = { candidateIds, targetId };
}

function openGroupMerge(group: TagDuplicateGroup) {
    openMerge(
        group.tags.map((t) => t.id),
        group.suggested_target_id,
    );
}

function openDeleteMany(ids: number[]) {
    clearMessages();
    deleteMany.value = tags.value.filter((t) => ids.includes(t.id));
}

function onDeletedMany(ids: number[]) {
    tags.value = tags.value.filter((t) => !ids.includes(t.id));
    list.value?.clearSelection();
    message.value = `${plural(ids.length, 'tag')} deleted`;
    refreshDuplicates();
}

async function onMerged(res: TagMergeResult) {
    merge.value = null;
    list.value?.clearSelection();
    message.value = `Merged ${plural(res.sources_deleted, 'tag')} into "${res.target.name}": ${plural(res.scenes_retagged, 'scene')} retagged`;
    await loadTags();
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

        <!-- Default sort -->
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Preferences</h3>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Default Sort
                </label>
                <UiSelectMenu
                    v-model="selectedTagSort"
                    :options="tagSortOptions"
                    class="max-w-48"
                />
                <p class="text-dim mt-1.5 text-[10px]">
                    Default sort order when opening the tag picker
                </p>
            </div>
        </div>

        <SettingsTagsCreateForm @created="onCreated" @error="onCreateError" />

        <SettingsTagsDuplicatesPanel :groups="duplicates" @merge="openGroupMerge" />

        <SettingsTagsManageList
            ref="list"
            :tags="tags"
            :loading="loading"
            @delete="handleDelete"
            @color="handleColor"
            @merge="openMerge"
            @delete-many="openDeleteMany"
            @renamed="onRenamed"
            @merge-into="(source, target) => openMerge([source.id, target.id], target.id)"
        />

        <SettingsTagsDeleteManyModal
            :visible="deleteMany !== null"
            :tags="deleteMany ?? []"
            @close="deleteMany = null"
            @deleted="onDeletedMany"
        />

        <SettingsTagsMergeModal
            :visible="merge !== null"
            :tags="tags"
            :candidate-ids="merge?.candidateIds ?? []"
            :initial-target-id="merge?.targetId ?? null"
            @close="merge = null"
            @merged="onMerged"
        />
    </div>
</template>
