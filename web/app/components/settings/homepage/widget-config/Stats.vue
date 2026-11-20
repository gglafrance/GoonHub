<script setup lang="ts">
// Config form of the stats widget: tiles shown, views period and timezone.
const config = defineModel<Record<string, unknown>>({ required: true });

const TILES = [
    { value: 'scenes', label: 'Scenes' },
    { value: 'hours', label: 'Hours of video' },
    { value: 'size', label: 'Library size' },
    { value: 'watched', label: 'Watched share' },
    { value: 'views', label: 'Views of the period' },
    { value: 'jizz', label: 'Jizz total' },
    { value: 'liked', label: 'Liked' },
    { value: 'markers', label: 'Markers' },
    { value: 'rating', label: 'Average rating' },
];

const PERIODS = [
    { value: 'month', label: 'Calendar month' },
    { value: 'week', label: 'Calendar week' },
    { value: '30d', label: 'Last 30 days' },
    { value: '7d', label: 'Last 7 days' },
];

const selectedTiles = computed<string[]>(() => {
    const raw = config.value.tiles;
    return Array.isArray(raw) && raw.length ? (raw as string[]) : TILES.map((t) => t.value);
});

const toggleTile = (value: string) => {
    const current = selectedTiles.value;
    const next = current.includes(value)
        ? current.filter((t) => t !== value)
        : TILES.map((t) => t.value).filter((t) => t === value || current.includes(t));
    if (next.length === 0) return;
    config.value = { ...config.value, tiles: next };
};

const period = computed({
    get: () => String(config.value.period ?? 'month'),
    set: (v: string) => (config.value = { ...config.value, period: v }),
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
                Tiles
            </label>
            <div class="flex flex-wrap gap-1.5">
                <button
                    v-for="tile in TILES"
                    :key="tile.value"
                    class="rounded-md border px-2 py-1 text-[11px] transition-all"
                    :class="
                        selectedTiles.includes(tile.value)
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    :aria-pressed="selectedTiles.includes(tile.value)"
                    @click="toggleTile(tile.value)"
                >
                    {{ tile.label }}
                </button>
            </div>
        </div>
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Views period
            </label>
            <UiSelectMenu v-model="period" :options="PERIODS" />
            <p class="text-dim mt-1 text-[10px]">Compared to the previous period.</p>
        </div>
        <SettingsHomepageWidgetConfigStatsSharedTimezoneField v-model="tz" />
    </div>
</template>
