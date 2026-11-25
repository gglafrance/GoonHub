<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// stats widget: library and personal figures as tiles.
interface StatsPayload {
    tiles: string[];
    period: 'month' | 'week' | '30d' | '7d';
    scene_count: number;
    total_duration: number;
    // Omitted without files:view_size
    total_size?: number;
    watched_scenes: number;
    watched_percent: number;
    views_period: number;
    views_prev_period: number;
    jizz_total: number;
    jizz_period: number;
    jizz_prev_period: number;
    last_jizz_at: string | null;
    jizz_per_week: number;
    liked_count: number;
    marker_count: number;
    rated_count: number;
    avg_rating: number | null;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<StatsPayload>;
}>();

defineEmits<{ refresh: [] }>();

const { formatSize } = useFormatter();
const { formatDate } = useDateFormatter();
const number = new Intl.NumberFormat();

const PERIOD_LABELS: Record<StatsPayload['period'], { current: string; previous: string }> = {
    month: { current: 'this month', previous: 'last month' },
    week: { current: 'this week', previous: 'last week' },
    '30d': { current: 'last 30 days', previous: 'previous 30 days' },
    '7d': { current: 'last 7 days', previous: 'previous 7 days' },
};

interface Tile {
    key: string;
    label: string;
    value: string;
    sub?: string;
    // Relative date shown as the value (NuxtTime)
    datetime?: string;
    trend?: 'up' | 'down' | 'flat';
    icon: string;
}

const tiles = computed<Tile[]>(() => {
    const p = props.data.widget;
    if (!p) return [];
    const period = PERIOD_LABELS[p.period] ?? PERIOD_LABELS.month;
    const delta = p.views_period - p.views_prev_period;
    const deltaPercent =
        p.views_prev_period > 0 ? Math.round((delta / p.views_prev_period) * 100) : null;
    const jizzDelta = p.jizz_period - p.jizz_prev_period;
    const jizzDeltaPercent =
        p.jizz_prev_period > 0 ? Math.round((jizzDelta / p.jizz_prev_period) * 100) : null;

    const all: Record<string, Tile> = {
        scenes: {
            key: 'scenes',
            label: 'Scenes',
            value: number.format(p.scene_count),
            icon: 'heroicons:film',
        },
        hours: {
            key: 'hours',
            label: 'Hours of video',
            value: number.format(Math.round(p.total_duration / 3600)),
            icon: 'heroicons:clock',
        },
        watched: {
            key: 'watched',
            label: 'Watched',
            value: `${p.watched_percent < 10 ? p.watched_percent.toFixed(1) : Math.round(p.watched_percent)}%`,
            sub: `${number.format(p.watched_scenes)} scenes seen`,
            icon: 'heroicons:eye',
        },
        views: {
            key: 'views',
            label: `Views ${period.current}`,
            value: number.format(p.views_period),
            sub:
                deltaPercent === null
                    ? `${number.format(p.views_prev_period)} ${period.previous}`
                    : `${delta >= 0 ? '+' : ''}${deltaPercent}% vs ${period.previous}`,
            trend: delta > 0 ? 'up' : delta < 0 ? 'down' : 'flat',
            icon: 'heroicons:play',
        },
        jizz: {
            key: 'jizz',
            label: 'Jizz total',
            value: number.format(p.jizz_total),
            icon: 'heroicons:fire',
        },
        jizz_period: {
            key: 'jizz_period',
            label: `O ${period.current}`,
            value: number.format(p.jizz_period),
            sub:
                jizzDeltaPercent === null
                    ? `${number.format(p.jizz_prev_period)} ${period.previous}`
                    : `${jizzDelta >= 0 ? '+' : ''}${jizzDeltaPercent}% vs ${period.previous}`,
            trend: jizzDelta > 0 ? 'up' : jizzDelta < 0 ? 'down' : 'flat',
            icon: 'heroicons:fire',
        },
        last_jizz: {
            key: 'last_jizz',
            label: 'Last O',
            value: p.last_jizz_at ? '' : '-',
            datetime: p.last_jizz_at ?? undefined,
            sub: p.last_jizz_at ? formatDate(p.last_jizz_at) : 'No dated O yet',
            icon: 'heroicons:clock',
        },
        jizz_per_week: {
            key: 'jizz_per_week',
            label: 'O per week',
            value:
                p.jizz_per_week < 10
                    ? p.jizz_per_week.toFixed(1)
                    : String(Math.round(p.jizz_per_week)),
            sub: 'average since the first dated O',
            icon: 'heroicons:chart-bar',
        },
        liked: {
            key: 'liked',
            label: 'Liked',
            value: number.format(p.liked_count),
            icon: 'heroicons:heart',
        },
        markers: {
            key: 'markers',
            label: 'Markers',
            value: number.format(p.marker_count),
            icon: 'heroicons:bookmark',
        },
        rating: {
            key: 'rating',
            label: 'Average rating',
            value: p.avg_rating !== null ? p.avg_rating.toFixed(2) : '-',
            sub: `${number.format(p.rated_count)} rated`,
            icon: 'heroicons:star',
        },
    };
    if (p.total_size !== undefined) {
        all.size = {
            key: 'size',
            label: 'Library size',
            value: formatSize(p.total_size),
            icon: 'heroicons:circle-stack',
        };
    }
    return p.tiles.map((key) => all[key]).filter((t): t is Tile => !!t);
});

const TREND_ICONS = {
    up: 'heroicons:arrow-trending-up',
    down: 'heroicons:arrow-trending-down',
    flat: 'heroicons:minus',
};
</script>

<template>
    <div
        v-if="tiles.length"
        class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5"
        role="list"
        aria-label="Library statistics"
    >
        <div
            v-for="tile in tiles"
            :key="tile.key"
            role="listitem"
            class="border-border rounded-lg border bg-white/2 p-3 transition-colors
                hover:border-white/20"
        >
            <div class="text-dim flex items-center gap-1.5 text-[10px] font-medium uppercase">
                <Icon :name="tile.icon" size="12" class="text-lava" />
                {{ tile.label }}
            </div>
            <div class="mt-1.5 font-mono text-xl font-semibold text-white tabular-nums">
                <NuxtTime v-if="tile.datetime" :datetime="tile.datetime" relative />
                <template v-else>{{ tile.value }}</template>
            </div>
            <div v-if="tile.sub" class="text-dim mt-0.5 flex items-center gap-1 text-[10px]">
                <Icon v-if="tile.trend" :name="TREND_ICONS[tile.trend]" size="11" />
                {{ tile.sub }}
            </div>
        </div>
    </div>
    <div
        v-else
        class="border-border text-dim flex h-24 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No statistics available
    </div>
</template>
