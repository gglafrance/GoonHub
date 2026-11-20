<script setup lang="ts">
// Config form of the hero (spotlight) widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const source = computed({
    get: () => String(config.value.source ?? 'liked'),
    set: (v: string | number) => (config.value = { ...config.value, source: String(v) }),
});

const rotateSeconds = computed({
    get: () => Number(config.value.rotate_seconds ?? 0),
    set: (v: string | number) => (config.value = { ...config.value, rotate_seconds: Number(v) }),
});

const sources = [
    { value: 'liked', label: 'Liked' },
    { value: 'top_jizz', label: 'Most jizzed' },
    { value: 'random', label: 'Random' },
    { value: 'unwatched', label: 'Unwatched' },
];

const rotations = [
    { value: 0, label: 'Manual' },
    { value: 10, label: '10 s' },
    { value: 20, label: '20 s' },
    { value: 60, label: '1 min' },
];
</script>

<template>
    <div class="space-y-4">
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="source"
            label="Featured scenes"
            :options="sources"
            hint="'Show N items' sets the size of the rotation pool"
        />
        <SettingsHomepageWidgetConfigDisplayOptionButtons
            v-model="rotateSeconds"
            label="Automatic rotation"
            :options="rotations"
            hint="Paused while the cursor is on the banner"
        />
    </div>
</template>
