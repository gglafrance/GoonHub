<script setup lang="ts">
// Config of the roulette widget: unwatched only, duration range, storage path, tags.
const config = defineModel<Record<string, unknown>>({ required: true });

const unwatchedOnly = computed({
    get: () => config.value.unwatched_only !== false,
    set: (v: boolean) => (config.value = { ...config.value, unwatched_only: v }),
});
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="unwatchedOnly"
            label="Unwatched only"
            description="Only draw scenes you have never watched"
        />
        <SettingsHomepageWidgetConfigScenesDurationRange v-model="config" />
        <SettingsHomepageWidgetConfigScenesStoragePathSelect v-model="config" />
        <SettingsHomepageWidgetConfigScenesTagPicker v-model="config" />
    </div>
</template>
