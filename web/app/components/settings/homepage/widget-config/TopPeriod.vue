<script setup lang="ts">
// Config form of the top_period widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const period = computed({
    get: () => String(config.value.period ?? 'month'),
    set: (v: string | number) => (config.value = { ...config.value, period: String(v) }),
});

const metric = computed({
    get: () => String(config.value.metric ?? 'views'),
    set: (v: string | number) => (config.value = { ...config.value, metric: String(v) }),
});

const metrics = [
    { value: 'views', label: 'Views' },
    { value: 'jizz', label: 'O' },
];

const periods = [
    { value: 'week', label: '7 days' },
    { value: 'month', label: '30 days' },
    { value: 'year', label: '365 days' },
];
</script>

<template>
    <div class="space-y-4">
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="metric"
            label="Rank by"
            :options="metrics"
            hint="O ranks by dated O only"
        />
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="period"
            label="Period"
            :options="periods"
            hint="Rolling period, compared to the period just before"
        />
    </div>
</template>
