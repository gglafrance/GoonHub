<script setup lang="ts">
// Config of the discover widget: duration range, storage path, tags.
const config = defineModel<Record<string, unknown>>({ required: true });
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigScenesDurationRange v-model="config" />
        <SettingsHomepageWidgetConfigScenesStoragePathSelect v-model="config" />
        <SettingsHomepageWidgetConfigScenesTagPicker v-model="config" />
    </div>
</template>
