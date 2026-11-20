<script setup lang="ts">
// Config form of the records widget: timezone of the day based records.
const config = defineModel<Record<string, unknown>>({ required: true });

const tz = computed({
    get: () => String(config.value.tz ?? ''),
    set: (v: string) => (config.value = { ...config.value, tz: v }),
});
</script>

<template>
    <SettingsHomepageWidgetConfigStatsSharedTimezoneField v-model="tz" />
</template>
