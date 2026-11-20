<script setup lang="ts">
// Config form of the activity_calendar widget: number of weeks, metric and timezone.
const config = defineModel<Record<string, unknown>>({ required: true });

const WEEK_OPTIONS = [12, 26, 52, 104];

const weeks = computed({
    get: () => Number(config.value.weeks ?? 52),
    set: (v: number) =>
        (config.value = { ...config.value, weeks: Math.min(104, Math.max(4, Math.round(v))) }),
});

const METRICS = [
    { value: 'views', label: 'Views' },
    { value: 'jizz', label: 'O' },
    { value: 'both', label: 'Both (toggle)' },
];

const metric = computed({
    get: () => String(config.value.metric ?? 'views'),
    set: (v: string) => (config.value = { ...config.value, metric: v }),
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
                Weeks shown
            </label>
            <div class="flex flex-wrap items-center gap-1.5">
                <button
                    v-for="option in WEEK_OPTIONS"
                    :key="option"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        weeks === option
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="weeks = option"
                >
                    {{ option === 104 ? '2 years' : option === 52 ? '1 year' : `${option} weeks` }}
                </button>
                <input
                    v-model.number="weeks"
                    type="number"
                    min="4"
                    max="104"
                    class="border-border bg-void/80 w-20 rounded-md border px-2 py-1 text-xs
                        text-white"
                />
            </div>
        </div>
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Metric
            </label>
            <div class="flex flex-wrap gap-1.5">
                <button
                    v-for="option in METRICS"
                    :key="option.value"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        metric === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    :aria-pressed="metric === option.value"
                    @click="metric = option.value"
                >
                    {{ option.label }}
                </button>
            </div>
            <p class="text-dim mt-1 text-[10px]">O only counts dated O.</p>
        </div>
        <SettingsHomepageWidgetConfigStatsSharedTimezoneField v-model="tz" />
    </div>
</template>
