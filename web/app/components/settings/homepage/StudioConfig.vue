<script setup lang="ts">
import type { Studio } from '~/types/studio';

const props = defineProps<{
    modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: Record<string, unknown>];
}>();

const { fetchStudios } = useApiStudios();

const studios = ref<Studio[]>([]);
const loading = ref(false);

const selectedStudioUUID = computed({
    get: () => (props.modelValue.studio_uuid as string) || '',
    set: (value: string) => {
        const studio = studios.value.find((s) => s.uuid === value);
        emit('update:modelValue', {
            ...props.modelValue,
            studio_uuid: value,
            studio_name: studio?.name || '',
        });
    },
});

onMounted(async () => {
    await loadStudios();
});

async function loadStudios() {
    loading.value = true;
    try {
        const res = await fetchStudios(1, 100);
        studios.value = res.data || [];
    } catch (e) {
        console.error('Failed to load studios:', e);
    } finally {
        loading.value = false;
    }
}

const studioOptions = computed(() =>
    studios.value.map((s) => ({
        value: s.uuid,
        label: s.name,
    })),
);
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Studio
        </label>
        <div v-if="loading" class="text-dim text-xs">Loading studios...</div>
        <div v-else-if="studios.length === 0" class="text-dim text-xs">No studios found</div>
        <UiSelectMenu v-else v-model="selectedStudioUUID" :options="studioOptions" searchable />
    </div>
</template>
