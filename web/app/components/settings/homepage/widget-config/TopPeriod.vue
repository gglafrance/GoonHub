<script setup lang="ts">
// Config form of the top_period widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const period = computed({
    get: () => String(config.value.period ?? 'month'),
    set: (v: string | number) => (config.value = { ...config.value, period: String(v) }),
});

const periods = [
    { value: 'week', label: '7 days' },
    { value: 'month', label: '30 days' },
    { value: 'year', label: '365 days' },
];
</script>

<template>
    <SettingsHomepageWidgetConfigDisplayOptionButtons
        v-model="period"
        label="Period"
        :options="periods"
        hint="Rolling period, compared to the period just before"
    />
</template>
