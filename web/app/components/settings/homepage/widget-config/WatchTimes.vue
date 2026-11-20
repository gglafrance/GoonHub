<script setup lang="ts">
// Config form of the watch_times widget: period and timezone.
const config = defineModel<Record<string, unknown>>({ required: true });

const PERIODS = [
    { value: 0, label: 'All history' },
    { value: 365, label: 'Last year' },
    { value: 90, label: 'Last 90 days' },
    { value: 30, label: 'Last 30 days' },
];

const days = computed({
    get: () => Number(config.value.days ?? 0),
    set: (v: number) => (config.value = { ...config.value, days: v }),
});

const tz = computed({
    get: () => String(config.value.tz ?? ''),
    set: (v: string) => (config.value = { ...config.value, tz: v }),
});
</script>

<template>
    <div class="space-y-4">
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Period
            </label>
            <div class="flex flex-wrap gap-1.5">
                <button
                    v-for="option in PERIODS"
                    :key="option.value"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        days === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="days = option.value"
                >
                    {{ option.label }}
                </button>
            </div>
        </div>
        <SettingsHomepageWidgetConfigStatsSharedTimezoneField v-model="tz" />
    </div>
</template>
