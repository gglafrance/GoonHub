<script setup lang="ts">
const authStore = useAuthStore();
const { getAppSettings, updateAppSettings } = useApiAdmin();

const props = defineProps<{
    activeSubTab: string;
}>();

const isAdmin = computed(() => authStore.user?.role === 'admin');

// Admin app settings state (lives here so it survives sub-tab switches)
const serveOGMetadata = ref(true);
const originalServeOGMetadata = ref(true);
const trashRetentionDays = ref(7);

const loadAppSettings = async () => {
    if (!isAdmin.value) return;
    try {
        const data = await getAppSettings();
        serveOGMetadata.value = data.serve_og_metadata;
        originalServeOGMetadata.value = data.serve_og_metadata;
        trashRetentionDays.value = data.trash_retention_days;
    } catch {
        // Silently fail - default values are already set
    }
};

const hasUnsavedAppSettings = computed(() => {
    if (!isAdmin.value) return false;
    return serveOGMetadata.value !== originalServeOGMetadata.value;
});

const saveAppSettings = async () => {
    await updateAppSettings({
        serve_og_metadata: serveOGMetadata.value,
        trash_retention_days: trashRetentionDays.value,
    });
    originalServeOGMetadata.value = serveOGMetadata.value;
};

defineExpose({ hasUnsavedAppSettings, saveAppSettings });

onMounted(() => {
    loadAppSettings();
});
</script>

<template>
    <div class="space-y-6">
        <SettingsAppGeneral v-if="props.activeSubTab === 'general'" />
        <SettingsAppSortDefaults v-if="props.activeSubTab === 'sort-defaults'" />
        <SettingsAppCardTemplate v-if="props.activeSubTab === 'card-template'" />
        <SettingsAppSearch v-if="props.activeSubTab === 'search'" />
        <SettingsAppAdvanced
            v-if="props.activeSubTab === 'advanced'"
            v-model:serve-og-metadata="serveOGMetadata"
        />
    </div>
</template>
