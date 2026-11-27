<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// jizz_insights widget: dated O per month (single lava series, best month labeled), headline
// figures, favorite hour and weekday, top scenes and the share of the library already O'd.
interface JizzInsightsScene {
    id: number;
    title: string;
    thumbnail_path: string;
    count: number;
}

interface JizzInsightsPayload {
    months: number;
    start: string;
    series: { month: string; count: number }[];
    total: number;
    avg_per_month: number;
    max: number;
    favorite_hour: number;
    favorite_weekday: number;
    top_scenes: JizzInsightsScene[];
    jizzed_scenes: number;
    library_count: number;
    jizzed_share: number;
    timezone: string;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<JizzInsightsPayload>;
}>();

defineEmits<{ refresh: [] }>();

const WEEKDAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const number = new Intl.NumberFormat();
const monthLong = new Intl.DateTimeFormat(undefined, {
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
});
const monthShort = new Intl.DateTimeFormat(undefined, {
    month: 'short',
    year: '2-digit',
    timeZone: 'UTC',
});
const parseMonth = (month: string) => new Date(`${month}-01T00:00:00Z`);
const pad = (h: number) => String(h).padStart(2, '0');

const widget = computed(() => props.data.widget);

// Label every Nth month so that about 8 labels fit
const bars = computed(() => {
    const series = widget.value?.series ?? [];
    const every = Math.max(1, Math.ceil(series.length / 8));
    const offset = (series.length - 1) % every; // keep the current month labeled
    return series.map((m, index) => ({
        key: m.month,
        label: (index - offset) % every === 0 ? monthShort.format(parseMonth(m.month)) : '',
        tooltip: `${monthLong.format(parseMonth(m.month))}: ${number.format(m.count)} O`,
        count: m.count,
        peak: m.count > 0 && m.count === widget.value?.max,
    }));
});

const figures = computed(() => {
    const p = widget.value;
    if (!p) return [];
    const avg = p.avg_per_month;
    return [
        { label: `O over ${p.months} months`, value: number.format(p.total) },
        { label: 'Per month', value: avg < 10 ? avg.toFixed(1) : number.format(Math.round(avg)) },
        {
            label: 'Favorite hour',
            value: p.favorite_hour >= 0 ? `${pad(p.favorite_hour)}:00` : '-',
        },
        {
            label: 'Favorite day',
            value: p.favorite_weekday >= 0 ? WEEKDAYS[p.favorite_weekday]! : '-',
        },
        {
            label: "Library O'd",
            value: `${p.jizzed_share < 10 ? p.jizzed_share.toFixed(1) : Math.round(p.jizzed_share)}%`,
            sub: `${number.format(p.jizzed_scenes)} / ${number.format(p.library_count)} scenes`,
        },
    ];
});
</script>

<template>
    <div v-if="widget" class="border-border rounded-lg border bg-white/2 p-4">
        <div class="mb-4 flex flex-wrap gap-x-6 gap-y-2">
            <div v-for="item in figures" :key="item.label">
                <div class="text-dim text-[10px] font-medium uppercase">{{ item.label }}</div>
                <div class="font-mono text-sm font-semibold text-white tabular-nums">
                    {{ item.value }}
                </div>
                <div v-if="item.sub" class="text-dim text-[10px]">{{ item.sub }}</div>
            </div>
        </div>

        <div v-if="widget.total === 0" class="text-dim py-6 text-center text-xs">
            No dated O over the last {{ widget.months }} months
        </div>
        <div v-else class="grid gap-6 lg:grid-cols-[2fr_1fr]">
            <HomepageWidgetsWatchTimesBars title="O per month" :bars="bars" />
            <HomepageWidgetsJizzInsightsTopScenes :scenes="widget.top_scenes" />
        </div>

        <div class="text-dim mt-2 text-[10px]">Timezone: {{ widget.timezone }}</div>
    </div>
</template>
