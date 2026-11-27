<script setup lang="ts">
const perms = useMyPermissions();
const { getAppSettings, updateAppSettings } = useApiAdmin();

const props = defineProps<{
    activeSubTab: string;
}>();

const canManageSettings = computed(() => perms.has('settings:manage'));

// Admin app settings state (lives here so it survives sub-tab switches)
const serveOGMetadata = ref(true);
const originalServeOGMetadata = ref(true);
const trashRetentionDays = ref(7);
const transcodingEnabled = ref(true);
const originalTranscodingEnabled = ref(true);

const loadAppSettings = async () => {
    if (!canManageSettings.value) return;
    try {
        const data = await getAppSettings();
        serveOGMetadata.value = data.serve_og_metadata;
        originalServeOGMetadata.value = data.serve_og_metadata;
        trashRetentionDays.value = data.trash_retention_days;
        transcodingEnabled.value = data.transcoding_enabled ?? true;
        originalTranscodingEnabled.value = transcodingEnabled.value;
    } catch {
        // Silently fail - default values are already set
    }
};

const hasUnsavedAppSettings = computed(() => {
    if (!canManageSettings.value) return false;
    return (
        serveOGMetadata.value !== originalServeOGMetadata.value ||
        transcodingEnabled.value !== originalTranscodingEnabled.value
    );
});

const saveAppSettings = async () => {
    await updateAppSettings({
        serve_og_metadata: serveOGMetadata.value,
        trash_retention_days: trashRetentionDays.value,
        transcoding_enabled: transcodingEnabled.value,
    });
    originalServeOGMetadata.value = serveOGMetadata.value;
    originalTranscodingEnabled.value = transcodingEnabled.value;
};

defineExpose({ hasUnsavedAppSettings, saveAppSettings });

onMounted(() => {
    loadAppSettings();
});
</script>

<template>
    <div class="space-y-6">
        <SettingsAppGeneral v-if="props.activeSubTab === 'general'" />
        <SettingsAppDisplay v-if="props.activeSubTab === 'display'" />
        <SettingsAppSortDefaults v-if="props.activeSubTab === 'sort-defaults'" />
        <SettingsAppCardTemplate v-if="props.activeSubTab === 'card-template'" />
        <SettingsAppSearch v-if="props.activeSubTab === 'search'" />
        <SettingsAppAdvanced
            v-if="props.activeSubTab === 'advanced'"
            v-model:serve-og-metadata="serveOGMetadata"
            v-model:transcoding-enabled="transcodingEnabled"
        />
    </div>
</template>
