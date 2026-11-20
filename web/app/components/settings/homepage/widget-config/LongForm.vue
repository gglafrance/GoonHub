<script setup lang="ts">
// Config of the long_form widget: minimum duration, in progress / unwatched, tags.
const config = defineModel<Record<string, unknown>>({ required: true });

const setKey = (key: string, value: unknown) => (config.value = { ...config.value, [key]: value });

const includeInProgress = computed({
    get: () => config.value.include_in_progress !== false,
    set: (v: boolean) => setKey('include_in_progress', v),
});
const includeUnwatched = computed({
    get: () => config.value.include_unwatched !== false,
    set: (v: boolean) => setKey('include_unwatched', v),
});
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigScenesDurationRange v-model="config" min-placeholder="20" />
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="includeInProgress"
            label="In progress"
            description="Scenes you started but did not finish (shown first)"
        />
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="includeUnwatched"
            label="Unwatched"
            description="Long scenes you have never watched"
        />
        <p v-if="!includeInProgress && !includeUnwatched" class="text-lava text-[11px]">
            Enable at least one of them.
        </p>
        <SettingsHomepageWidgetConfigScenesTagPicker v-model="config" />
    </div>
</template>
