<script setup lang="ts">
import type { SavedMarkerFilter, SavedMarkerFilterValues } from '~/types/marker';

// Discreet "Saved" dropdown of the marker pages: apply, save, rename and delete named filters.
const props = defineProps<{
    // Current filters to save (with sort/view when the page has them)
    snapshot: () => SavedMarkerFilterValues;
}>();

const emit = defineEmits<{
    apply: [values: SavedMarkerFilterValues];
}>();

const {
    fetchSavedMarkerFilters,
    createSavedMarkerFilter,
    updateSavedMarkerFilter,
    deleteSavedMarkerFilter,
} = useApiMarkers();

const open = ref(false);
const filters = ref<SavedMarkerFilter[]>([]);
const loaded = ref(false);
const error = ref<string | null>(null);
const newName = ref('');
const renamingId = ref<string | null>(null);
const renameValue = ref('');
const confirmDeleteId = ref<string | null>(null);
const root = ref<HTMLElement | null>(null);

const load = async () => {
    try {
        filters.value = (await fetchSavedMarkerFilters()).filters;
        loaded.value = true;
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to load saved filters';
    }
};

const toggle = () => {
    open.value = !open.value;
    if (open.value && !loaded.value) load();
};

const apply = (filter: SavedMarkerFilter) => {
    emit('apply', filter.filters);
    open.value = false;
};

const save = async () => {
    const name = newName.value.trim();
    if (!name) return;
    error.value = null;
    try {
        const created = await createSavedMarkerFilter(name, props.snapshot());
        filters.value = [...filters.value, created].sort((a, b) => a.name.localeCompare(b.name));
        newName.value = '';
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to save';
    }
};

const startRename = (filter: SavedMarkerFilter) => {
    renamingId.value = filter.uuid;
    renameValue.value = filter.name;
};

const confirmRename = async (filter: SavedMarkerFilter) => {
    const name = renameValue.value.trim();
    renamingId.value = null;
    if (!name || name === filter.name) return;
    try {
        const updated = await updateSavedMarkerFilter(filter.uuid, { name });
        filters.value = filters.value.map((f) => (f.uuid === updated.uuid ? updated : f));
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to rename';
    }
};

const remove = async (filter: SavedMarkerFilter) => {
    if (confirmDeleteId.value !== filter.uuid) {
        confirmDeleteId.value = filter.uuid;
        return;
    }
    confirmDeleteId.value = null;
    try {
        await deleteSavedMarkerFilter(filter.uuid);
        filters.value = filters.value.filter((f) => f.uuid !== filter.uuid);
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to delete';
    }
};

const onClickOutside = (event: MouseEvent) => {
    if (open.value && root.value && !root.value.contains(event.target as Node)) {
        open.value = false;
    }
};

onMounted(() => document.addEventListener('mousedown', onClickOutside));
onUnmounted(() => document.removeEventListener('mousedown', onClickOutside));
</script>

<template>
    <div ref="root" class="relative shrink-0">
        <button
            class="border-border bg-panel text-dim flex h-full items-center gap-1.5 rounded-lg
                border px-2.5 py-1.5 text-[11px] font-medium transition-all hover:text-white"
            :class="{ 'border-lava/30 text-lava': open }"
            title="Saved marker filters"
            @click="toggle"
        >
            <Icon name="heroicons:bookmark" size="14" />
            <span class="hidden sm:inline">Saved</span>
        </button>

        <div
            v-if="open"
            class="border-border bg-panel absolute top-full right-0 z-30 mt-1 w-64 rounded-lg border
                p-1.5 shadow-xl"
        >
            <div v-if="!loaded && !error" class="text-dim px-2 py-1.5 text-[11px]">Loading...</div>
            <div
                v-else-if="loaded && filters.length === 0"
                class="text-dim px-2 py-1.5 text-[11px]"
            >
                No saved filters yet
            </div>
            <div class="max-h-60 overflow-y-auto">
                <div
                    v-for="filter in filters"
                    :key="filter.uuid"
                    class="group flex items-center gap-1 rounded-md px-1.5 py-1 hover:bg-white/5"
                >
                    <input
                        v-if="renamingId === filter.uuid"
                        v-model="renameValue"
                        maxlength="100"
                        class="border-border bg-void min-w-0 flex-1 rounded border px-1.5 py-0.5
                            text-[11px] text-white focus:outline-none"
                        @keydown.enter="confirmRename(filter)"
                        @keydown.escape="renamingId = null"
                        @blur="confirmRename(filter)"
                    />
                    <button
                        v-else
                        class="min-w-0 flex-1 truncate text-left text-[11px] text-white/80
                            hover:text-white"
                        :title="`Apply ${filter.name}`"
                        @click="apply(filter)"
                    >
                        {{ filter.name }}
                    </button>
                    <button
                        class="text-dim p-0.5 opacity-0 transition-opacity group-hover:opacity-100
                            hover:text-white"
                        title="Rename"
                        @click="startRename(filter)"
                    >
                        <Icon name="heroicons:pencil" size="11" />
                    </button>
                    <button
                        class="p-0.5 transition-opacity group-hover:opacity-100"
                        :class="
                            confirmDeleteId === filter.uuid
                                ? 'text-lava opacity-100'
                                : 'text-dim hover:text-lava opacity-0'
                        "
                        :title="
                            confirmDeleteId === filter.uuid ? 'Click again to delete' : 'Delete'
                        "
                        @click="remove(filter)"
                    >
                        <Icon name="heroicons:trash" size="11" />
                    </button>
                </div>
            </div>
            <div class="border-border mt-1 flex items-center gap-1 border-t px-0.5 pt-1.5">
                <input
                    v-model="newName"
                    maxlength="100"
                    placeholder="Save current filters as..."
                    class="border-border bg-void placeholder:text-dim/60 min-w-0 flex-1 rounded-md
                        border px-2 py-1 text-[11px] text-white focus:outline-none"
                    @keydown.enter="save"
                />
                <button
                    class="bg-lava/15 text-lava hover:bg-lava/25 rounded-md px-2 py-1 text-[11px]
                        font-medium transition-colors disabled:opacity-40"
                    :disabled="!newName.trim()"
                    @click="save"
                >
                    Save
                </button>
            </div>
            <p v-if="error" class="text-lava px-1.5 pt-1 text-[10px]">{{ error }}</p>
        </div>
    </div>
</template>
