<script setup lang="ts">
// Config form of the quick_rate widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const days = computed({
    get: () => Number(config.value.days ?? 30),
    set: (v: string | number) => (config.value = { ...config.value, days: Number(v) }),
});

const options = [
    { value: 7, label: '7 days' },
    { value: 30, label: '30 days' },
    { value: 90, label: '90 days' },
    { value: 365, label: '1 year' },
];
</script>

<template>
    <SettingsHomepageWidgetConfigDisplayOptionButtons
        v-model="days"
        label="Watched within"
        :options="options"
        hint="Scenes you watched in this period and have not rated yet"
    />
</template>
