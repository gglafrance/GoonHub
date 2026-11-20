<script setup lang="ts">
// Config form of the shorts widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const maxDuration = computed({
    get: () => Number(config.value.max_duration ?? 60),
    set: (v: string | number) => (config.value = { ...config.value, max_duration: Number(v) }),
});

const unwatchedOnly = computed({
    get: () => config.value.unwatched_only === true,
    set: (v: boolean) => (config.value = { ...config.value, unwatched_only: v }),
});

const durations = [
    { value: 15, label: '15 s' },
    { value: 30, label: '30 s' },
    { value: 60, label: '1 min' },
    { value: 120, label: '2 min' },
    { value: 300, label: '5 min' },
];
</script>

<template>
    <div class="space-y-4">
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="maxDuration"
            label="Maximum duration"
            :options="durations"
        />
        <div class="flex items-center justify-between">
            <div>
                <span class="block text-xs font-medium text-white">Unwatched only</span>
                <span class="text-dim text-[11px]">Only clips you have never watched</span>
            </div>
            <UiToggle v-model="unwatchedOnly" />
        </div>
    </div>
</template>
