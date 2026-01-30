<script setup lang="ts">
import type { Actor } from '~/types/actor';

const props = defineProps<{
    modelValue: Record<string, unknown>;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: Record<string, unknown>];
}>();

const { fetchActors } = useApiActors();

const actors = ref<Actor[]>([]);
const loading = ref(false);
const searchQuery = ref('');

const selectedActorUUID = computed({
    get: () => (props.modelValue.actor_uuid as string) || '',
    set: (value: string) => {
        const actor = actors.value.find((a) => a.uuid === value);
        emit('update:modelValue', {
            ...props.modelValue,
            actor_uuid: value,
            actor_name: actor?.name || '',
        });
    },
});

onMounted(async () => {
    await loadActors();
});

async function loadActors() {
    loading.value = true;
    try {
        const res = await fetchActors(1, 100);
        actors.value = res.data || [];
    } catch (e) {
        console.error('Failed to load actors:', e);
    } finally {
        loading.value = false;
    }
}

const filteredActors = computed(() => {
    if (!searchQuery.value) return actors.value;
    const q = searchQuery.value.toLowerCase();
    return actors.value.filter((a) => a.name.toLowerCase().includes(q));
});

const actorOptions = computed(() =>
    filteredActors.value.map((a) => ({
        value: a.uuid,
        label: a.name,
    })),
);
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Actor
        </label>
        <div v-if="loading" class="text-dim text-xs">Loading actors...</div>
        <div v-else-if="actors.length === 0" class="text-dim text-xs">No actors found</div>
        <UiSelectMenu v-else v-model="selectedActorUUID" :options="actorOptions" searchable />
    </div>
</template>
