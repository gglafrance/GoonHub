<script setup lang="ts">
// Config of the rediscover widget: unseen period and which favorites to include.
const config = defineModel<Record<string, unknown>>({ required: true });

const setKey = (key: string, value: unknown) => (config.value = { ...config.value, [key]: value });

const days = computed({
    get: () => Number(config.value.days ?? 180),
    set: (v: number) => setKey('days', Math.min(3650, Math.max(1, Math.round(v || 180)))),
});
const minRating = computed({
    get: () => Number(config.value.min_rating ?? 4),
    set: (v: number) => setKey('min_rating', Math.min(5, Math.max(0, v || 0))),
});
const includeLiked = computed({
    get: () => config.value.include_liked !== false,
    set: (v: boolean) => setKey('include_liked', v),
});
const includeJizzed = computed({
    get: () => config.value.include_jizzed !== false,
    set: (v: boolean) => setKey('include_jizzed', v),
});

const periods = [
    { days: 30, label: '1 month' },
    { days: 90, label: '3 months' },
    { days: 180, label: '6 months' },
    { days: 365, label: '1 year' },
];
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigScenesField label="Not watched for">
            <div class="flex flex-wrap items-center gap-1.5">
                <button
                    v-for="p in periods"
                    :key="p.days"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        days === p.days
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="days = p.days"
                >
                    {{ p.label }}
                </button>
                <input
                    v-model.number="days"
                    type="number"
                    min="1"
                    max="3650"
                    class="border-border bg-void/80 w-20 rounded-md border px-2 py-1 text-xs
                        text-white"
                />
                <span class="text-dim text-[11px]">days</span>
            </div>
        </SettingsHomepageWidgetConfigScenesField>
        <SettingsHomepageWidgetConfigScenesField
            label="Minimum rating"
            hint="Scenes rated at least this are included (0: ratings are ignored)."
        >
            <input
                v-model.number="minRating"
                type="number"
                min="0"
                max="5"
                step="0.5"
                class="border-border bg-void/80 w-24 rounded-lg border px-3 py-2 text-xs text-white"
            />
        </SettingsHomepageWidgetConfigScenesField>
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="includeLiked"
            label="Liked scenes"
            description="Include the scenes you liked"
        />
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="includeJizzed"
            label="Jizzed scenes"
            description="Include the scenes with a jizz count"
        />
    </div>
</template>
