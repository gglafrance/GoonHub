<script setup lang="ts">
// Config form of the jizz_insights widget: number of months and timezone.
const config = defineModel<Record<string, unknown>>({ required: true });

const MONTH_OPTIONS = [
    { value: 6, label: '6 months' },
    { value: 12, label: '1 year' },
    { value: 24, label: '2 years' },
    { value: 60, label: '5 years' },
];

const months = computed({
    get: () => Number(config.value.months ?? 24),
    set: (v: number) =>
        (config.value = { ...config.value, months: Math.min(120, Math.max(3, Math.round(v))) }),
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
                Months shown
            </label>
            <div class="flex flex-wrap items-center gap-1.5">
                <button
                    v-for="option in MONTH_OPTIONS"
                    :key="option.value"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        months === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    :aria-pressed="months === option.value"
                    @click="months = option.value"
                >
                    {{ option.label }}
                </button>
                <input
                    v-model.number="months"
                    type="number"
                    min="3"
                    max="120"
                    class="border-border bg-void/80 w-20 rounded-md border px-2 py-1 text-xs
                        text-white"
                />
            </div>
            <p class="text-dim mt-1 text-[10px]">Current month included. Only dated O count.</p>
        </div>
        <SettingsHomepageWidgetConfigStatsSharedTimezoneField v-model="tz" />
    </div>
</template>
