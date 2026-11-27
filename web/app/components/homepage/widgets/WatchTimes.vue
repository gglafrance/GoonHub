<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// watch_times widget: views or dated O by hour of day and by day of week (toggle).
interface WatchTimesSeries {
    hours: number[];
    weekdays: number[];
    total: number;
    peak_hour: number;
    peak_weekday: number;
}

interface WatchTimesPayload extends WatchTimesSeries {
    jizz?: WatchTimesSeries;
    metric?: 'views' | 'jizz';
    days: number;
    timezone: string;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<WatchTimesPayload>;
}>();

defineEmits<{ refresh: [] }>();

const WEEKDAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const number = new Intl.NumberFormat();

const pad = (h: number) => String(h).padStart(2, '0');

const metric = ref<'views' | 'jizz'>(props.data.widget?.metric === 'jizz' ? 'jizz' : 'views');
watch(
    () => props.data.widget?.metric,
    (m) => (metric.value = m === 'jizz' ? 'jizz' : 'views'),
);
const hasJizz = computed(() => !!props.data.widget?.jizz);
const series = computed<WatchTimesSeries | null>(() => {
    const p = props.data.widget;
    if (!p) return null;
    return metric.value === 'jizz' && p.jizz ? p.jizz : p;
});
const unit = computed(() => (metric.value === 'jizz' ? 'O' : 'views'));

const hourBars = computed(() =>
    (series.value?.hours ?? []).map((count, hour) => ({
        key: `h${hour}`,
        label: hour % 6 === 0 ? `${pad(hour)}h` : '',
        tooltip: `${pad(hour)}:00-${pad((hour + 1) % 24)}:00: ${number.format(count)} ${unit.value}`,
        count,
        peak: hour === series.value?.peak_hour && count > 0,
    })),
);

const weekdayBars = computed(() =>
    (series.value?.weekdays ?? []).map((count, day) => ({
        key: `d${day}`,
        label: WEEKDAYS[day]!.slice(0, 3),
        tooltip: `${WEEKDAYS[day]}: ${number.format(count)} ${unit.value}`,
        count,
        peak: day === series.value?.peak_weekday && count > 0,
    })),
);

const peakText = computed(() => {
    const p = series.value;
    if (!p || p.total === 0) return '';
    return `Peak: ${pad(p.peak_hour)}:00-${pad((p.peak_hour + 1) % 24)}:00 · ${WEEKDAYS[p.peak_weekday]}`;
});

const periodText = computed(() => {
    const days = props.data.widget?.days ?? 0;
    return days > 0 ? `last ${days} days` : 'all history';
});
</script>

<template>
    <div v-if="data.widget && series" class="border-border rounded-lg border bg-white/2 p-4">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div class="text-xs text-white">{{ peakText }}</div>
            <div class="text-dim flex items-center gap-3 text-[10px]">
                <span>
                    {{ number.format(series.total) }} {{ unit }} · {{ periodText }} ·
                    {{ data.widget.timezone }}
                </span>
                <HomepageWidgetsMetricToggle v-if="hasJizz" v-model="metric" />
            </div>
        </div>
        <div v-if="series.total === 0" class="text-dim py-6 text-center text-xs">
            {{ metric === 'jizz' ? 'No dated O yet' : 'No views yet' }}
        </div>
        <template v-else>
            <div class="grid gap-6 lg:grid-cols-[2fr_1fr]">
                <HomepageWidgetsWatchTimesBars title="By hour of day" :bars="hourBars" />
                <HomepageWidgetsWatchTimesBars title="By day of week" :bars="weekdayBars" />
            </div>
        </template>
    </div>
</template>
