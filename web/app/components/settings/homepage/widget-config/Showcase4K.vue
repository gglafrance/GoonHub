<script setup lang="ts">
// Config of the showcase_4k widget: minimum resolution, order, unwatched only, tags.
const config = defineModel<Record<string, unknown>>({ required: true });

const setKey = (key: string, value: unknown) => (config.value = { ...config.value, [key]: value });

const resolutions = [
    { value: 2160, label: '4K' },
    { value: 1440, label: '1440p' },
    { value: 1080, label: '1080p' },
];
const orders = [
    { value: 'random', label: 'Random' },
    { value: 'newest', label: 'Newest' },
];

const minHeight = computed({
    get: () => Number(config.value.min_height ?? 2160),
    set: (v: number) => setKey('min_height', v),
});
const order = computed({
    get: () => String(config.value.order ?? 'random'),
    set: (v: string) => setKey('order', v),
});
const unwatchedOnly = computed({
    get: () => config.value.unwatched_only === true,
    set: (v: boolean) => setKey('unwatched_only', v),
});
</script>

<template>
    <div class="space-y-3">
        <div class="flex gap-4">
            <SettingsHomepageWidgetConfigScenesField label="Minimum resolution">
                <div class="flex gap-1.5">
                    <button
                        v-for="r in resolutions"
                        :key="r.value"
                        class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                        :class="
                            minHeight === r.value
                                ? 'border-lava/40 bg-lava/10 text-lava'
                                : 'border-border text-dim hover:text-white'
                        "
                        @click="minHeight = r.value"
                    >
                        {{ r.label }}
                    </button>
                </div>
            </SettingsHomepageWidgetConfigScenesField>
            <SettingsHomepageWidgetConfigScenesField label="Order">
                <div class="flex gap-1.5">
                    <button
                        v-for="o in orders"
                        :key="o.value"
                        class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                        :class="
                            order === o.value
                                ? 'border-lava/40 bg-lava/10 text-lava'
                                : 'border-border text-dim hover:text-white'
                        "
                        @click="order = o.value"
                    >
                        {{ o.label }}
                    </button>
                </div>
            </SettingsHomepageWidgetConfigScenesField>
        </div>
        <SettingsHomepageWidgetConfigScenesToggleRow
            v-model="unwatchedOnly"
            label="Unwatched only"
            description="Hide the scenes you already watched"
        />
        <SettingsHomepageWidgetConfigScenesTagPicker v-model="config" />
    </div>
</template>
