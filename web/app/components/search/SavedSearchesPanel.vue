<script setup lang="ts">
import type { SavedSearch, SavedSearchFilters } from '~/types/saved_search';

const emit = defineEmits<{
    load: [filters: SavedSearchFilters];
}>();

const { fetchSavedSearches, updateSavedSearch, deleteSavedSearch } = useApiSavedSearches();

const searches = ref<SavedSearch[]>([]);
const isLoading = ref(false);
const error = ref('');
const expanded = ref(true);

// Edit state
const editingUuid = ref<string | null>(null);
const editName = ref('');
const editLoading = ref(false);

const loadSearches = async () => {
    isLoading.value = true;
    error.value = '';
    try {
        const response = await fetchSavedSearches();
        searches.value = response.data;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load saved searches';
    } finally {
        isLoading.value = false;
    }
};

onMounted(loadSearches);

const getFilterSummary = (filters: SavedSearchFilters): string => {
    const parts: string[] = [];

    if (filters.query) parts.push(`"${filters.query}"`);
    if (filters.selected_tags?.length) parts.push(`${filters.selected_tags.length} tags`);
    if (filters.selected_actors?.length) parts.push(`${filters.selected_actors.length} actors`);
    if (filters.studio) parts.push(filters.studio);
    if (filters.resolution) parts.push(filters.resolution);
    if (filters.liked) parts.push('Liked');

    return parts.length > 0 ? parts.slice(0, 3).join(', ') : 'No filters';
};

const handleLoad = (search: SavedSearch) => {
    emit('load', search.filters);
};

const startEdit = (search: SavedSearch) => {
    editingUuid.value = search.uuid;
    editName.value = search.name;
};

const cancelEdit = () => {
    editingUuid.value = null;
    editName.value = '';
};

const saveEdit = async (uuid: string) => {
    if (!editName.value.trim()) return;

    editLoading.value = true;
    try {
        await updateSavedSearch(uuid, { name: editName.value.trim() });
        const search = searches.value.find((s) => s.uuid === uuid);
        if (search) {
            search.name = editName.value.trim();
        }
        editingUuid.value = null;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update';
    } finally {
        editLoading.value = false;
    }
};

const handleDelete = async (uuid: string) => {
    try {
        await deleteSavedSearch(uuid);
        searches.value = searches.value.filter((s) => s.uuid !== uuid);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete';
    }
};

defineExpose({ reload: loadSearches });
</script>

<template>
    <div class="mb-4">
        <button
            class="mb-2 flex w-full items-center justify-between text-left"
            @click="expanded = !expanded"
        >
            <h3 class="text-xs font-semibold tracking-wide text-white uppercase">Saved Searches</h3>
            <Icon
                :name="expanded ? 'heroicons:chevron-up' : 'heroicons:chevron-down'"
                size="14"
                class="text-dim transition-colors hover:text-white"
            />
        </button>

        <div v-if="expanded">
            <div v-if="isLoading" class="text-dim py-2 text-center text-xs">Loading...</div>

            <div
                v-else-if="error"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-2 py-1.5 text-xs"
            >
                {{ error }}
            </div>

            <div v-else-if="searches.length === 0" class="text-dim py-2 text-center text-xs">
                No saved searches yet
            </div>

            <div v-else class="space-y-1">
                <div
                    v-for="search in searches"
                    :key="search.uuid"
                    class="bg-surface/50 border-border group rounded-lg border p-2 transition-all
                        hover:border-white/20"
                >
                    <div v-if="editingUuid === search.uuid" class="flex items-center gap-2">
                        <input
                            v-model="editName"
                            type="text"
                            maxlength="255"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                min-w-0 flex-1 rounded border px-2 py-1 text-xs text-white
                                focus:ring-1 focus:outline-none"
                            @keyup.enter="saveEdit(search.uuid)"
                            @keyup.escape="cancelEdit"
                        />
                        <button
                            :disabled="editLoading || !editName.trim()"
                            class="text-lava hover:text-lava/80 disabled:opacity-50"
                            @click="saveEdit(search.uuid)"
                        >
                            <Icon name="heroicons:check" size="14" />
                        </button>
                        <button class="text-dim hover:text-white" @click="cancelEdit">
                            <Icon name="heroicons:x-mark" size="14" />
                        </button>
                    </div>

                    <div v-else>
                        <div class="flex items-start justify-between gap-2">
                            <button
                                class="min-w-0 flex-1 text-left"
                                :title="'Load: ' + search.name"
                                @click="handleLoad(search)"
                            >
                                <div class="truncate text-xs font-medium text-white">
                                    {{ search.name }}
                                </div>
                                <div class="text-dim mt-0.5 truncate text-[10px]">
                                    {{ getFilterSummary(search.filters) }}
                                </div>
                            </button>

                            <div
                                class="flex shrink-0 items-center gap-1 opacity-0 transition-opacity
                                    group-hover:opacity-100"
                            >
                                <button
                                    class="text-dim hover:text-white"
                                    title="Rename"
                                    @click.stop="startEdit(search)"
                                >
                                    <Icon name="heroicons:pencil" size="12" />
                                </button>
                                <button
                                    class="text-dim hover:text-lava"
                                    title="Delete"
                                    @click.stop="handleDelete(search.uuid)"
                                >
                                    <Icon name="heroicons:trash" size="12" />
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
