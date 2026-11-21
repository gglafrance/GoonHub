<script setup lang="ts">
// Min/max duration inputs (minutes) bound to the min_duration/max_duration config keys
// (seconds). Empty means no limit.
const config = defineModel<Record<string, unknown>>({ required: true });

defineProps<{
    hideMax?: boolean;
    minPlaceholder?: string;
}>();

const toMinutes = (key: string) => {
    const v = Number(config.value[key]);
    return v > 0 ? Math.round((v / 60) * 10) / 10 : '';
};

const setMinutes = (key: string, raw: string) => {
    const minutes = Number(raw);
    const next = Object.fromEntries(Object.entries(config.value).filter(([k]) => k !== key));
    if (raw !== '' && minutes > 0) next[key] = Math.round(minutes * 60);
    config.value = next;
};
</script>

<template>
    <div class="flex gap-3">
        <SettingsHomepageWidgetConfigScenesField label="Min duration (min)" class="flex-1">
            <input
                type="number"
                min="0"
                step="0.5"
                :value="toMinutes('min_duration')"
                :placeholder="minPlaceholder ?? 'Any'"
                class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3
                    py-2 text-xs text-white focus:outline-none"
                @change="setMinutes('min_duration', ($event.target as HTMLInputElement).value)"
            />
        </SettingsHomepageWidgetConfigScenesField>
        <SettingsHomepageWidgetConfigScenesField
            v-if="!hideMax"
            label="Max duration (min)"
            class="flex-1"
        >
            <input
                type="number"
                min="0"
                step="0.5"
                :value="toMinutes('max_duration')"
                placeholder="Any"
                class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3
                    py-2 text-xs text-white focus:outline-none"
                @change="setMinutes('max_duration', ($event.target as HTMLInputElement).value)"
            />
        </SettingsHomepageWidgetConfigScenesField>
    </div>
</template>
