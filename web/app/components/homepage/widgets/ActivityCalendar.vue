<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// activity_calendar widget: GitHub-style heatmap of views and/or O per day, with streaks.
// With metric "both", a toggle switches between the two series (same lava ramp, never two
// hues on one grid).
interface CalendarSeries {
    days: { day: string; count: number }[];
    total: number;
    active_days: number;
    max: number;
    current_streak: number;
    longest_streak: number;
}

type CalendarMetric = 'views' | 'jizz';

interface CalendarPayload extends CalendarSeries {
    weeks: number;
    start: string;
    today: string;
    metric?: CalendarMetric | 'both';
    series?: Partial<Record<CalendarMetric, CalendarSeries>>;
    timezone: string;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<CalendarPayload>;
}>();

defineEmits<{ refresh: [] }>();

const CELL = 11;
const GAP = 2;
const STEP = CELL + GAP;
const LEFT = 26; // weekday labels
const TOP = 14; // month labels
const WEEKDAY_LABELS = ['Mon', '', 'Wed', '', 'Fri', '', ''];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

const number = new Intl.NumberFormat();

const available = computed<CalendarMetric[]>(() => {
    const series = props.data.widget?.series ?? {};
    const list = (['views', 'jizz'] as CalendarMetric[]).filter((m) => series[m]);
    return list.length ? list : ['views'];
});
const active = ref<CalendarMetric>('views');
watch(
    available,
    (list) => {
        if (!list.includes(active.value)) active.value = list[0]!;
    },
    { immediate: true },
);

// Series shown: the selected one, else the top level figures (older payloads)
const current = computed<CalendarSeries | null>(() => {
    const p = props.data.widget;
    if (!p) return null;
    return p.series?.[active.value] ?? p;
});

const UNITS: Record<CalendarMetric, { one: string; many: string; title: string }> = {
    views: { one: 'view', many: 'views', title: 'Views' },
    jizz: { one: 'O', many: 'O', title: 'O' },
};
const unit = computed(() => UNITS[active.value]);

// Dates are handled as UTC calendar days to avoid local DST shifts
const parseDay = (day: string) => new Date(`${day}T00:00:00Z`);
const formatDay = (d: Date) => d.toISOString().slice(0, 10);
const { formatDate } = useDateFormatter();
const weekdayFormat = new Intl.DateTimeFormat(undefined, { weekday: 'short', timeZone: 'UTC' });
const longDate = {
    format: (d: Date) => `${weekdayFormat.format(d)} ${formatDate(d, { utc: true })}`,
};

interface Cell {
    day: string;
    count: number;
    x: number;
    y: number;
    level: number;
}

const counts = computed(() => {
    const map = new Map<string, number>();
    for (const d of current.value?.days ?? []) map.set(d.day, d.count);
    return map;
});

// Intensity level 0-4 (quartiles of the max)
const levelOf = (count: number, max: number) => {
    if (count <= 0 || max <= 0) return 0;
    return Math.min(4, Math.max(1, Math.ceil((count / max) * 4)));
};

const grid = computed(() => {
    const p = props.data.widget;
    const max = current.value?.max ?? 0;
    if (!p) return { cells: [] as Cell[], months: [] as { x: number; label: string }[], weeks: 0 };
    const start = parseDay(p.start);
    const today = parseDay(p.today);
    const cells: Cell[] = [];
    const months: { x: number; label: string }[] = [];
    let lastMonth = -1;
    for (let w = 0; w < p.weeks; w++) {
        for (let d = 0; d < 7; d++) {
            const date = new Date(start.getTime() + (w * 7 + d) * 86400000);
            if (date > today) break;
            const day = formatDay(date);
            const count = counts.value.get(day) ?? 0;
            cells.push({
                day,
                count,
                x: LEFT + w * STEP,
                y: TOP + d * STEP,
                level: levelOf(count, max),
            });
            if (d === 0 && date.getUTCMonth() !== lastMonth) {
                lastMonth = date.getUTCMonth();
                // Skip a label too close to the left edge on a partial first month
                if (w > 0 || date.getUTCDate() <= 7) {
                    months.push({ x: LEFT + w * STEP, label: MONTHS[lastMonth]! });
                }
            }
        }
    }
    return { cells, months, weeks: p.weeks };
});

const width = computed(() => LEFT + grid.value.weeks * STEP);
const height = TOP + 7 * STEP;

// Sequential lava ramp (one hue, opacity steps) on the dark surface
const LEVEL_CLASSES = ['fill-white/5', 'fill-lava/25', 'fill-lava/45', 'fill-lava/70', 'fill-lava'];

const hovered = ref<Cell | null>(null);
const tooltipStyle = ref<Record<string, string>>({});

const showTooltip = (cell: Cell, event: MouseEvent) => {
    hovered.value = cell;
    const target = (event.currentTarget as SVGElement).getBoundingClientRect();
    tooltipStyle.value = {
        left: `${target.left + target.width / 2}px`,
        top: `${target.top - 6}px`,
    };
};

const tooltipText = (cell: Cell) =>
    `${longDate.format(parseDay(cell.day))}: ${
        cell.count === 0
            ? `no ${unit.value.many}`
            : `${number.format(cell.count)} ${cell.count > 1 ? unit.value.many : unit.value.one}`
    }`;

const summary = computed(() => {
    const p = current.value;
    if (!p) return [];
    return [
        { label: unit.value.title, value: number.format(p.total) },
        { label: 'Active days', value: number.format(p.active_days) },
        { label: 'Current streak', value: `${p.current_streak} d` },
        { label: 'Longest streak', value: `${p.longest_streak} d` },
    ];
});

const ariaLabel = computed(() => {
    const p = current.value;
    const weeks = props.data.widget?.weeks ?? 0;
    if (!p) return 'Activity calendar';
    return `${unit.value.title} per day over the last ${weeks} weeks: ${p.total} ${unit.value.many} on ${p.active_days} days, current streak ${p.current_streak} days, longest streak ${p.longest_streak} days`;
});
</script>

<template>
    <div v-if="data.widget" class="border-border rounded-lg border bg-white/2 p-4">
        <div class="mb-3 flex flex-wrap items-start gap-x-6 gap-y-2">
            <div v-for="item in summary" :key="item.label">
                <div class="text-dim text-[10px] font-medium uppercase">{{ item.label }}</div>
                <div class="font-mono text-sm font-semibold text-white tabular-nums">
                    {{ item.value }}
                </div>
            </div>
            <HomepageWidgetsMetricToggle
                v-if="available.length > 1"
                v-model="active"
                :options="available"
                class="ml-auto"
            />
        </div>

        <div class="overflow-x-auto">
            <svg
                :viewBox="`0 0 ${width} ${height}`"
                :width="width"
                :height="height"
                class="max-w-none"
                role="img"
                :aria-label="ariaLabel"
                @mouseleave="hovered = null"
            >
                <text
                    v-for="month in grid.months"
                    :key="month.x"
                    :x="month.x"
                    y="9"
                    class="fill-white/40 text-[9px]"
                >
                    {{ month.label }}
                </text>
                <text
                    v-for="(label, index) in WEEKDAY_LABELS"
                    :key="index"
                    x="0"
                    :y="TOP + index * STEP + CELL - 2"
                    class="fill-white/40 text-[9px]"
                >
                    {{ label }}
                </text>
                <rect
                    v-for="cell in grid.cells"
                    :key="cell.day"
                    :x="cell.x"
                    :y="cell.y"
                    :width="CELL"
                    :height="CELL"
                    rx="2"
                    :class="[
                        LEVEL_CLASSES[cell.level],
                        hovered?.day === cell.day ? 'stroke-white/70' : 'stroke-transparent',
                    ]"
                    stroke-width="1"
                    @mouseenter="showTooltip(cell, $event)"
                >
                    <title>{{ tooltipText(cell) }}</title>
                </rect>
            </svg>
        </div>

        <div class="text-dim mt-2 flex items-center justify-between text-[10px]">
            <span>Timezone: {{ data.widget.timezone }}</span>
            <div class="flex items-center gap-1" aria-hidden="true">
                Less
                <svg
                    v-for="(cls, index) in LEVEL_CLASSES"
                    :key="index"
                    width="10"
                    height="10"
                    viewBox="0 0 10 10"
                >
                    <rect width="10" height="10" rx="2" :class="cls" />
                </svg>
                More
            </div>
        </div>

        <Teleport to="body">
            <div
                v-if="hovered"
                class="border-border bg-panel/95 pointer-events-none fixed z-50 -translate-x-1/2
                    -translate-y-full rounded-md border px-2 py-1 text-[11px] whitespace-nowrap
                    text-white shadow-lg backdrop-blur-md"
                :style="tooltipStyle"
            >
                {{ tooltipText(hovered) }}
            </div>
        </Teleport>
    </div>
</template>
