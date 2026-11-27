<script setup lang="ts">
import type { StoragePath } from '~/types/storage';

// Storage path selector bound to the storage_path_id config key (admin API: other users only
// get the "Any" option).
const config = defineModel<Record<string, unknown>>({ required: true });

defineProps<{
    label?: string;
    allowAny?: boolean;
}>();

const { fetchStoragePaths } = useApiStorage();
const paths = ref<StoragePath[]>([]);

onMounted(async () => {
    try {
        const data = await fetchStoragePaths();
        paths.value = data.storage_paths ?? [];
    } catch {
        paths.value = [];
    }
});

const selected = computed({
    get: () => String(config.value.storage_path_id ?? ''),
    set: (v: string) => {
        const next = { ...config.value };
        if (v) next.storage_path_id = Number(v);
        else delete next.storage_path_id;
        config.value = next;
    },
});
</script>

<template>
    <SettingsHomepageWidgetConfigScenesField :label="label ?? 'Storage path'">
        <select
            v-model="selected"
            class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3 py-2
                text-xs text-white focus:outline-none"
        >
            <option v-if="allowAny !== false" value="">Any</option>
            <option v-for="path in paths" :key="path.id" :value="String(path.id)">
                {{ path.name }} ({{ path.path }})
            </option>
        </select>
    </SettingsHomepageWidgetConfigScenesField>
</template>
