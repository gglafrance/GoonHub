<script setup lang="ts">
// Config of the on_this_day widget: watched, added or both.
const config = defineModel<Record<string, unknown>>({ required: true });

const modes = [
    { value: 'both', label: 'Watched & added' },
    { value: 'watched', label: 'Watched' },
    { value: 'added', label: 'Added' },
];

const mode = computed({
    get: () => String(config.value.mode ?? 'both'),
    set: (v: string) => (config.value = { ...config.value, mode: v }),
});
</script>

<template>
    <SettingsHomepageWidgetConfigScenesField
        label="Show scenes"
        hint="Scenes watched or added on today's date in previous years."
    >
        <div class="flex gap-1.5">
            <button
                v-for="m in modes"
                :key="m.value"
                class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                :class="
                    mode === m.value
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : 'border-border text-dim hover:text-white'
                "
                @click="mode = m.value"
            >
                {{ m.label }}
            </button>
        </div>
    </SettingsHomepageWidgetConfigScenesField>
</template>
