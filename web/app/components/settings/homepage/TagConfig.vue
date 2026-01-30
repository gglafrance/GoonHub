<script setup lang="ts">
import type { Tag } from '~/types/tag';

const props = defineProps<{
    modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: Record<string, unknown>];
}>();

const { fetchTags } = useApiTags();

const tags = ref<Tag[]>([]);
const loading = ref(false);

const selectedTagId = computed({
    get: () => Number(props.modelValue.tag_id) || 0,
    set: (value: number | string) => {
        const numValue = Number(value);
        const tag = tags.value.find((t) => t.id === numValue);
        emit('update:modelValue', {
            ...props.modelValue,
            tag_id: numValue,
            tag_name: tag?.name || '',
        });
    },
});

onMounted(async () => {
    await loadTags();
});

async function loadTags() {
    loading.value = true;
    try {
        const res = await fetchTags();
        tags.value = res.data || [];
    } catch (e) {
        console.error('Failed to load tags:', e);
    } finally {
        loading.value = false;
    }
}

const tagOptions = computed(() =>
    tags.value.map((t) => ({
        value: t.id,
        label: t.name,
    })),
);
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Tag
        </label>
        <div v-if="loading" class="text-dim text-xs">Loading tags...</div>
        <div v-else-if="tags.length === 0" class="text-dim text-xs">No tags found</div>
        <UiSelectMenu v-else v-model="selectedTagId" :options="tagOptions" searchable />
    </div>
</template>
