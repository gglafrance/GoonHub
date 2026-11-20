<script setup lang="ts">
// Config form of the preview_wall widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const source = computed({
    get: () => String(config.value.source ?? 'random'),
    set: (v: string | number) => (config.value = { ...config.value, source: String(v) }),
});

const columns = computed({
    get: () => Number(config.value.columns ?? 4),
    set: (v: string | number) => (config.value = { ...config.value, columns: Number(v) }),
});

const sources = [
    { value: 'random', label: 'Random' },
    { value: 'unwatched', label: 'Unwatched' },
    { value: 'liked', label: 'Liked' },
    { value: 'latest', label: 'Latest' },
];

const columnOptions = [3, 4, 5, 6].map((n) => ({ value: n, label: String(n) }));
</script>

<template>
    <div class="space-y-4">
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="source"
            label="Scenes"
            :options="sources"
            hint="Scenes with a generated preview are shown first"
        />
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="columns"
            label="Columns"
            :options="columnOptions"
        />
    </div>
</template>
