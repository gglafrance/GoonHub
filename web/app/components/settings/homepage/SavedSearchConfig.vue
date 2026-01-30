<script setup lang="ts">
import type { SavedSearch } from '~/types/saved_search';

const props = defineProps<{
    modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: Record<string, unknown>];
}>();

const { fetchSavedSearches } = useApiSavedSearches();

const savedSearches = ref<SavedSearch[]>([]);
const loading = ref(false);

const selectedSearchUUID = computed({
    get: () => (props.modelValue.saved_search_uuid as string) || '',
    set: (value: string) => {
        const search = savedSearches.value.find((s) => s.uuid === value);
        emit('update:modelValue', {
            ...props.modelValue,
            saved_search_uuid: value,
            saved_search_name: search?.name || '',
        });
    },
});

onMounted(async () => {
    await loadSavedSearches();
});

async function loadSavedSearches() {
    loading.value = true;
    try {
        const res = await fetchSavedSearches();
        savedSearches.value = res.data || [];
    } catch (e) {
        console.error('Failed to load saved searches:', e);
    } finally {
        loading.value = false;
    }
}

const searchOptions = computed(() =>
    savedSearches.value.map((s) => ({
        value: s.uuid,
        label: s.name,
    })),
);
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Saved Search
        </label>
        <div v-if="loading" class="text-dim text-xs">Loading saved searches...</div>
        <div v-else-if="savedSearches.length === 0" class="text-dim text-xs">
            No saved searches found. Create one from the search page first.
        </div>
        <UiSelectMenu v-else v-model="selectedSearchUUID" :options="searchOptions" />
    </div>
</template>
