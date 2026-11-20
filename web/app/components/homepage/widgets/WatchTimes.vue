<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// watch_times widget: views by hour of day and by day of week.
interface WatchTimesPayload {
    hours: number[];
    weekdays: number[];
    total: number;
    peak_hour: number;
    peak_weekday: number;
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

const hourBars = computed(() =>
    (props.data.widget?.hours ?? []).map((count, hour) => ({
        key: `h${hour}`,
        label: hour % 6 === 0 ? `${pad(hour)}h` : '',
        tooltip: `${pad(hour)}:00-${pad((hour + 1) % 24)}:00: ${number.format(count)} views`,
        count,
        peak: hour === props.data.widget?.peak_hour && count > 0,
    })),
);

const weekdayBars = computed(() =>
    (props.data.widget?.weekdays ?? []).map((count, day) => ({
        key: `d${day}`,
        label: WEEKDAYS[day]!.slice(0, 3),
        tooltip: `${WEEKDAYS[day]}: ${number.format(count)} views`,
        count,
        peak: day === props.data.widget?.peak_weekday && count > 0,
    })),
);

const peakText = computed(() => {
    const p = props.data.widget;
    if (!p || p.total === 0) return '';
    return `Peak: ${pad(p.peak_hour)}:00-${pad((p.peak_hour + 1) % 24)}:00 · ${WEEKDAYS[p.peak_weekday]}`;
});

const periodText = computed(() => {
    const days = props.data.widget?.days ?? 0;
    return days > 0 ? `last ${days} days` : 'all history';
});
</script>

<template>
    <div v-if="data.widget" class="border-border rounded-lg border bg-white/2 p-4">
        <div v-if="data.widget.total === 0" class="text-dim py-6 text-center text-xs">
            No views yet
        </div>
        <template v-else>
            <div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
                <div class="text-xs text-white">{{ peakText }}</div>
                <div class="text-dim text-[10px]">
                    {{ number.format(data.widget.total) }} views · {{ periodText }} ·
                    {{ data.widget.timezone }}
                </div>
            </div>
            <div class="grid gap-6 lg:grid-cols-[2fr_1fr]">
                <HomepageWidgetsWatchTimesBars
                    title="By hour of day"
                    :bars="hourBars"
                />
                <HomepageWidgetsWatchTimesBars
                    title="By day of week"
                    :bars="weekdayBars"
                />
            </div>
        </template>
    </div>
</template>
