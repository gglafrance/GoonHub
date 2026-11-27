<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// top_period widget: most watched (or most o'd) scenes over the period, with the count and the
// change compared to the previous period. With metric jizz, the "views" fields hold o counts.
interface TopPeriodItem {
    scene_id: number;
    views: number;
    previous_views: number;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{
        period: 'week' | 'month' | 'year';
        metric?: 'views' | 'jizz';
        total_views: number;
        previous_total_views: number;
        items: TopPeriodItem[];
    }>;
}>();

const { formatDuration } = useFormatter();

const PERIOD_LABELS = {
    week: { current: 'last 7 days', previous: 'previous 7 days' },
    month: { current: 'last 30 days', previous: 'previous 30 days' },
    year: { current: 'last 365 days', previous: 'previous 365 days' },
} as const;

const period = computed(() => PERIOD_LABELS[props.data.widget?.period ?? 'month']);
const isJizz = computed(() => props.data.widget?.metric === 'jizz');
const unit = computed(() => (isJizz.value ? 'O' : 'views'));

const items = computed(() => {
    const byId = new Map(props.data.widget?.items?.map((i) => [i.scene_id, i]) ?? []);
    return props.data.scenes.map((scene, rank) => {
        const item = byId.get(scene.id);
        return {
            scene,
            rank: rank + 1,
            views: item?.views ?? 0,
            delta: (item?.views ?? 0) - (item?.previous_views ?? 0),
            isNew: (item?.previous_views ?? 0) === 0,
        };
    });
});

const total = computed(() => props.data.widget?.total_views ?? 0);
const previousTotal = computed(() => props.data.widget?.previous_total_views ?? 0);
const change = computed(() => {
    if (previousTotal.value === 0) return null;
    return Math.round(((total.value - previousTotal.value) / previousTotal.value) * 100);
});
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-4">
        <div class="mb-3 flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span class="text-2xl font-bold text-white">{{ total }}</span>
            <span class="text-dim text-xs">{{ unit }} over the {{ period.current }}</span>
            <span
                v-if="change !== null"
                class="rounded-full px-1.5 py-0.5 font-mono text-[10px]"
                :class="change >= 0 ? 'bg-emerald/10 text-emerald' : 'bg-lava/10 text-lava'"
                :title="`${previousTotal} ${unit} over the ${period.previous}`"
            >
                {{ change >= 0 ? '+' : '' }}{{ change }}%
            </span>
        </div>

        <div v-if="items.length === 0" class="text-dim py-6 text-center text-xs">
            {{ isJizz ? 'No dated O' : 'Nothing watched' }} over the {{ period.current }}
        </div>
        <ol v-else class="grid gap-x-6 gap-y-1 md:grid-cols-2">
            <li v-for="item in items" :key="item.scene.id">
                <NuxtLink
                    :to="`/watch/${item.scene.id}`"
                    class="group flex items-center gap-3 rounded-md px-1.5 py-1.5 transition-colors
                        hover:bg-white/5"
                >
                    <span
                        class="w-6 shrink-0 text-right font-mono text-sm font-bold"
                        :class="item.rank <= 3 ? 'text-lava' : 'text-dim'"
                    >
                        {{ item.rank }}
                    </span>
                    <img
                        :src="`/thumbnails/${item.scene.id}`"
                        :alt="item.scene.title"
                        loading="lazy"
                        class="h-9 w-16 shrink-0 rounded bg-black/50 object-cover"
                    />
                    <div class="min-w-0 flex-1">
                        <div class="truncate text-xs text-white/90 group-hover:text-white">
                            {{ item.scene.title }}
                        </div>
                        <div class="text-dim font-mono text-[10px]">
                            {{ formatDuration(item.scene.duration) }}
                        </div>
                    </div>
                    <div class="shrink-0 text-right">
                        <div class="font-mono text-xs text-white">
                            {{ item.views }}{{ isJizz ? ' O' : 'x' }}
                        </div>
                        <div
                            class="font-mono text-[10px]"
                            :class="
                                item.isNew
                                    ? 'text-cyan-400'
                                    : item.delta > 0
                                      ? 'text-emerald'
                                      : item.delta < 0
                                        ? 'text-lava'
                                        : 'text-dim'
                            "
                        >
                            {{
                                item.isNew
                                    ? 'new'
                                    : item.delta === 0
                                      ? '='
                                      : `${item.delta > 0 ? '+' : ''}${item.delta}`
                            }}
                        </div>
                    </div>
                </NuxtLink>
            </li>
        </ol>
    </div>
</template>
