<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// records widget: viewing records and library extremes.
interface RecordScene {
    id: number;
    title: string;
    thumbnail_path: string;
    value: number;
}

interface RecordsPayload {
    most_rewatched: RecordScene | null;
    most_jizzed: RecordScene | null;
    longest_scene: RecordScene | null;
    biggest_file: RecordScene | null;
    longest_streak: { days: number; start: string; end: string } | null;
    busiest_day: { day: string; count: number } | null;
    first_view: string | null;
    total_watch_time: number;
    total_views: number;
    timezone: string;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<RecordsPayload>;
}>();

defineEmits<{ refresh: [] }>();

const { formatDuration, formatSize } = useFormatter();
const number = new Intl.NumberFormat();
const dayFormat = new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
});
const formatDay = (day: string) => dayFormat.format(new Date(`${day}T00:00:00Z`));

const formatHours = (seconds: number) => {
    const hours = seconds / 3600;
    return hours >= 10 ? `${number.format(Math.round(hours))} h` : `${hours.toFixed(1)} h`;
};

interface RecordItem {
    key: string;
    label: string;
    icon: string;
    value: string;
    detail?: string;
    scene?: RecordScene;
}

const items = computed<RecordItem[]>(() => {
    const p = props.data.widget;
    if (!p) return [];
    const list: RecordItem[] = [];
    if (p.most_rewatched) {
        list.push({
            key: 'most_rewatched',
            label: 'Most rewatched',
            icon: 'heroicons:arrow-path',
            value: `${number.format(p.most_rewatched.value)} views`,
            scene: p.most_rewatched,
        });
    }
    if (p.most_jizzed) {
        list.push({
            key: 'most_jizzed',
            label: 'Most jizzed',
            icon: 'heroicons:fire',
            value: `${number.format(p.most_jizzed.value)} times`,
            scene: p.most_jizzed,
        });
    }
    if (p.longest_streak) {
        list.push({
            key: 'longest_streak',
            label: 'Longest streak',
            icon: 'heroicons:bolt',
            value: `${p.longest_streak.days} days`,
            detail: `${formatDay(p.longest_streak.start)} - ${formatDay(p.longest_streak.end)}`,
        });
    }
    if (p.busiest_day) {
        list.push({
            key: 'busiest_day',
            label: 'Busiest day',
            icon: 'heroicons:calendar',
            value: `${number.format(p.busiest_day.count)} views`,
            detail: formatDay(p.busiest_day.day),
        });
    }
    if (p.longest_scene) {
        list.push({
            key: 'longest_scene',
            label: 'Longest scene',
            icon: 'heroicons:film',
            value: formatDuration(p.longest_scene.value),
            scene: p.longest_scene,
        });
    }
    if (p.biggest_file) {
        list.push({
            key: 'biggest_file',
            label: 'Biggest file',
            icon: 'heroicons:circle-stack',
            value: formatSize(p.biggest_file.value),
            scene: p.biggest_file,
        });
    }
    if (p.first_view) {
        list.push({
            key: 'first_view',
            label: 'First view',
            icon: 'heroicons:flag',
            value: dayFormat.format(new Date(p.first_view)),
            detail: `${number.format(p.total_views)} views since`,
        });
    }
    if (p.total_watch_time > 0) {
        list.push({
            key: 'total_watch_time',
            label: 'Time watched',
            icon: 'heroicons:clock',
            value: formatHours(p.total_watch_time),
        });
    }
    return list;
});
</script>

<template>
    <div v-if="items.length" class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
        <component
            :is="item.scene ? resolveComponent('NuxtLink') : 'div'"
            v-for="item in items"
            :key="item.key"
            :to="item.scene ? `/watch/${item.scene.id}` : undefined"
            class="border-border group flex items-center gap-3 rounded-lg border bg-white/2 p-3
                transition-colors"
            :class="item.scene ? 'hover:border-lava/30' : ''"
        >
            <div
                v-if="item.scene?.thumbnail_path"
                class="h-10 w-16 shrink-0 overflow-hidden rounded bg-black/50"
            >
                <img
                    :src="`/thumbnails/${item.scene.id}?size=sm`"
                    :alt="item.scene.title"
                    class="h-full w-full object-cover"
                    loading="lazy"
                />
            </div>
            <div
                v-else
                class="bg-lava/10 flex h-10 w-10 shrink-0 items-center justify-center rounded"
            >
                <Icon :name="item.icon" size="16" class="text-lava" />
            </div>
            <div class="min-w-0">
                <div class="text-dim text-[10px] font-medium uppercase">{{ item.label }}</div>
                <div class="font-mono text-sm font-semibold text-white tabular-nums">
                    {{ item.value }}
                </div>
                <div
                    v-if="item.scene || item.detail"
                    class="text-dim truncate text-[10px] transition-colors"
                    :class="item.scene ? 'group-hover:text-lava' : ''"
                    :title="item.scene?.title ?? item.detail"
                >
                    {{ item.scene?.title || item.detail }}
                </div>
            </div>
        </component>
    </div>
    <div
        v-else
        class="border-border text-dim flex h-24 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No records yet
    </div>
</template>
