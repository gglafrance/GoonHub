<script setup lang="ts">
import type { ProfileBlock, ProfileStatsPayload, ProfileTopEntry } from '~/types/profile';

// stats: watch figures of the period and totals, with the top tags, actors and studios by
// views (profile block). No file information.
const props = defineProps<{ block: ProfileBlock<ProfileStatsPayload>; owner: boolean }>();

const number = new Intl.NumberFormat();
const PERIODS: Record<string, string> = {
    week: 'last 7 days',
    month: 'last 30 days',
    year: 'last 365 days',
    all: 'all time',
};

const stats = computed(() => props.block.widget);
const tiles = computed(() => {
    const s = stats.value;
    if (!s) return [];
    return [
        { label: 'Watch time', value: formatWatchTime(s.watch_time) },
        { label: 'Views', value: number.format(s.views) },
        { label: 'Videos watched', value: number.format(s.scenes_watched) },
        { label: 'O', value: number.format(s.jizz) },
        { label: 'Liked', value: number.format(s.liked), total: true },
        { label: 'Markers', value: number.format(s.markers), total: true },
        {
            label: 'Avg rating',
            value: s.avg_rating != null ? s.avg_rating.toFixed(1) : '-',
            total: true,
        },
    ];
});

const tops = computed(() => {
    const s = stats.value;
    if (!s) return [];
    const link = (kind: string, e: ProfileTopEntry) =>
        kind === 'tags'
            ? `/search?tags=${encodeURIComponent(e.name)}`
            : e.uuid
              ? `/${kind}/${e.uuid}`
              : undefined;
    return [
        { title: 'Top tags', kind: 'tags', items: s.top_tags },
        { title: 'Top actors', kind: 'actors', items: s.top_actors },
        { title: 'Top studios', kind: 'studios', items: s.top_studios },
    ]
        .filter((t) => t.items.length > 0)
        .map((t) => ({ ...t, items: t.items.map((e) => ({ ...e, to: link(t.kind, e) })) }));
});
</script>

<template>
    <div v-if="stats" class="space-y-4">
        <div class="text-dim text-[10px] tracking-wider uppercase">
            {{ PERIODS[stats.period] ?? stats.period }}
        </div>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-7">
            <div
                v-for="tile in tiles"
                :key="tile.label"
                class="border-border rounded-lg border bg-white/[0.02] px-3 py-2"
            >
                <div class="font-mono text-base font-semibold text-white">{{ tile.value }}</div>
                <div class="text-dim text-[10px]">
                    {{ tile.label
                    }}<span v-if="tile.total && stats.period !== 'all'"> (total)</span>
                </div>
            </div>
        </div>
        <div v-if="tops.length" class="grid gap-3 sm:grid-cols-3">
            <div v-for="top in tops" :key="top.kind">
                <div class="text-dim mb-1.5 text-[10px] tracking-wider uppercase">
                    {{ top.title }}
                </div>
                <ol class="space-y-1">
                    <li
                        v-for="(entry, i) in top.items"
                        :key="entry.id"
                        class="flex items-center gap-2 text-xs"
                    >
                        <span class="text-dim w-3 font-mono text-[10px]">{{ i + 1 }}</span>
                        <NuxtLink
                            v-if="entry.to"
                            :to="entry.to"
                            class="hover:text-lava min-w-0 flex-1 truncate text-white"
                        >
                            {{ entry.name }}
                        </NuxtLink>
                        <span v-else class="min-w-0 flex-1 truncate text-white">
                            {{ entry.name }}
                        </span>
                        <span class="text-dim font-mono text-[10px]">{{ entry.views }}</span>
                    </li>
                </ol>
            </div>
        </div>
    </div>
</template>
