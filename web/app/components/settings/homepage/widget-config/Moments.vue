<script setup lang="ts">
// Config form of the moments widget.
const config = defineModel<Record<string, unknown>>({ required: true });
</script>

<template>
    <SettingsHomepageWidgetConfigMarkersMarkerFilters v-model="config" with-order />
</template>
