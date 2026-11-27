<script setup lang="ts">
// Config form of the recent_markers widget.
const config = defineModel<Record<string, unknown>>({ required: true });
</script>

<template>
    <SettingsHomepageWidgetConfigMarkersMarkerFilters v-model="config" />
</template>
