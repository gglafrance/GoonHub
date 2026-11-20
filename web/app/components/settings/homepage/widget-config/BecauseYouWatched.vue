<script setup lang="ts">
// Config of the because_you_watched widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const excludeWatched = computed({
    get: () => config.value.exclude_watched !== false,
    set: (v: boolean) => (config.value = { ...config.value, exclude_watched: v }),
});
</script>

<template>
    <SettingsHomepageWidgetConfigScenesToggleRow
        v-model="excludeWatched"
        label="Hide watched scenes"
        description="Only suggest similar scenes you have not watched yet"
    />
</template>
