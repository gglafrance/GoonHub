<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// trends widget: tags and actors whose share of your views grew the most recently.
interface TrendItem {
    kind: 'tag' | 'actor';
    id: number;
    uuid?: string;
    name: string;
    color?: string;
    recent_views: number;
    base_views: number;
    recent_share: number;
    base_share: number;
    change_pct?: number;
    new: boolean;
}

interface TrendsPayload {
    recent_days: number;
    baseline_days: number;
    recent_views: number;
    base_views: number;
    items: TrendItem[];
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<TrendsPayload>;
}>();

defineEmits<{ refresh: [] }>();

const payload = computed(() => props.data.widget ?? null);
const items = computed(() => payload.value?.items ?? []);

// Largest recent share, to scale the bars
const maxShare = computed(() => Math.max(0.0001, ...items.value.map((i) => i.recent_share)));

const linkFor = (item: TrendItem) =>
    item.kind === 'actor' && item.uuid
        ? `/actors/${item.uuid}`
        : `/search?${item.kind === 'actor' ? 'actors' : 'tags'}=${encodeURIComponent(item.name)}`;

const percent = (share: number) => `${(share * 100).toFixed(share < 0.1 ? 1 : 0)}%`;

const changeLabel = (item: TrendItem) => {
    if (item.new || item.change_pct === undefined) return 'new';
    const pct = Math.round(item.change_pct);
    return `+${pct >= 1000 ? `${Math.round(pct / 100) / 10}k` : pct}%`;
};
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-4">
        <div
            v-if="!payload || payload.recent_views === 0"
            class="text-dim py-6 text-center text-xs"
        >
            No views in the last {{ payload?.recent_days ?? 30 }} days yet.
        </div>
        <template v-else>
            <p class="text-dim mb-3 text-[11px]">
                Share of your {{ payload.recent_views }} views over the last
                {{ payload.recent_days }} days, compared to the {{ payload.baseline_days }} days
                before.
            </p>
            <div v-if="items.length === 0" class="text-dim py-4 text-center text-xs">
                Nothing is rising compared to the previous period.
            </div>
            <ul v-else class="space-y-1.5">
                <li v-for="item in items" :key="`${item.kind}-${item.id}`">
                    <NuxtLink
                        :to="linkFor(item)"
                        class="group flex items-center gap-3 rounded-md px-2 py-1.5
                            transition-colors hover:bg-white/5"
                    >
                        <Icon
                            :name="item.kind === 'actor' ? 'heroicons:user' : 'heroicons:tag'"
                            size="13"
                            class="text-dim shrink-0"
                            :style="item.color ? { color: item.color } : undefined"
                        />
                        <div class="min-w-0 flex-1">
                            <div class="flex items-center justify-between gap-2">
                                <span class="truncate text-xs text-white/90 group-hover:text-white">
                                    {{ item.name }}
                                </span>
                                <span
                                    class="shrink-0 rounded px-1.5 py-0.5 font-mono text-[10px]"
                                    :class="
                                        item.new
                                            ? 'bg-lava/15 text-lava'
                                            : 'bg-emerald/10 text-emerald'
                                    "
                                >
                                    {{ changeLabel(item) }}
                                </span>
                            </div>
                            <div class="mt-1 flex items-center gap-2">
                                <div class="relative h-1 flex-1 rounded-full bg-white/10">
                                    <div
                                        class="absolute h-full rounded-full bg-white/25"
                                        :style="{ width: `${(item.base_share / maxShare) * 100}%` }"
                                        title="Previous period share"
                                    />
                                    <div
                                        class="bg-lava absolute h-full rounded-full opacity-80"
                                        :style="{
                                            width: `${(item.recent_share / maxShare) * 100}%`,
                                        }"
                                        title="Recent share"
                                    />
                                </div>
                                <span
                                    class="text-dim w-24 shrink-0 text-right font-mono text-[10px]"
                                >
                                    {{ percent(item.base_share) }} &rarr;
                                    {{ percent(item.recent_share) }}
                                </span>
                            </div>
                        </div>
                    </NuxtLink>
                </li>
            </ul>
        </template>
    </div>
</template>
