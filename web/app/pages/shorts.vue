<script setup lang="ts">
import type { ClipsFilter, ShortsSort } from '~/types/shorts';

useHead({ title: 'Shorts' });

definePageMeta({
    middleware: ['auth'],
});

const SORT_KEY = 'goonhub:shorts:sort';
const CLIPS_KEY = 'goonhub:shorts:clips';

const readStored = <T extends string>(key: string, allowed: readonly T[], fallback: T): T => {
    try {
        const v = localStorage.getItem(key) as T | null;
        return v && allowed.includes(v) ? v : fallback;
    } catch {
        return fallback;
    }
};

const route = useRoute();
const router = useRouter();

const sort = ref<ShortsSort>('newest');
const clips = ref<ClipsFilter>('all');
const ready = ref(false);

// One list for the grid and the player, so both show the same order.
const feed = useShortsFeed(sort, clips);
const { items, activeIndex } = feed;

// The player shows while ?play=<scene id> names a loaded short.
const playId = computed(() => {
    const v = Number(route.query.play);
    return Number.isInteger(v) && v > 0 ? v : null;
});
const playIndex = computed(() =>
    playId.value === null ? -1 : items.value.findIndex((i) => i.id === playId.value),
);
const playing = computed(() => playIndex.value >= 0);
// The short to bring into view when the grid comes back
const lastIndex = ref<number | null>(null);

const withoutPlay = () => {
    const { play: _play, ...rest } = route.query;
    return rest;
};

watch(playing, (now, before) => {
    if (now) activeIndex.value = playIndex.value;
    else if (before) lastIndex.value = activeIndex.value;
});

// Keep the URL on the short in view, without adding history entries.
watch(activeIndex, (i) => {
    const item = items.value[i];
    if (playing.value && item && item.id !== playId.value) {
        router.replace({ query: { ...route.query, play: String(item.id) } });
    }
});

const open = (index: number) => {
    const item = items.value[index];
    if (!item) return;
    activeIndex.value = index;
    router.push({ query: { ...route.query, play: String(item.id) } });
};

const close = () => {
    lastIndex.value = activeIndex.value;
    // Back to the grid entry when we came from it; otherwise drop ?play
    if (window.history.state?.back) router.back();
    else router.replace({ query: withoutPlay() });
};

onMounted(async () => {
    sort.value = readStored(SORT_KEY, ['newest', 'random'] as const, 'newest');
    clips.value = readStored(CLIPS_KEY, ['all', 'only', 'hide'] as const, 'all');
    ready.value = true;
    await feed.reset();
    // A link to a short past the first page opens on the grid instead
    if (playId.value !== null && !playing.value) router.replace({ query: withoutPlay() });
});

watch([sort, clips], () => {
    if (!ready.value) return;
    lastIndex.value = null;
    if (playId.value !== null) router.replace({ query: withoutPlay() });
    feed.reset();
});

watch(sort, (v) => {
    try {
        localStorage.setItem(SORT_KEY, v);
    } catch {
        // Storage unavailable: the choice lasts for this visit only
    }
});
watch(clips, (v) => {
    try {
        localStorage.setItem(CLIPS_KEY, v);
    } catch {
        // Storage unavailable: the choice lasts for this visit only
    }
});

const clipOptions: { value: ClipsFilter; label: string }[] = [
    { value: 'all', label: 'All shorts' },
    { value: 'only', label: 'Only created clips' },
    { value: 'hide', label: 'Hide created clips' },
];
</script>

<template>
    <div
        class="mx-auto flex flex-col gap-2 px-2 py-2 sm:px-4"
        :class="playing ? 'h-[calc(100dvh-5.6rem)] max-w-3xl' : 'max-w-7xl'"
    >
        <div class="flex shrink-0 items-center gap-2">
            <button
                v-if="playing"
                class="border-border bg-panel text-dim flex items-center gap-1 rounded-lg border
                    px-2 py-1.5 text-[11px] font-medium transition-colors hover:text-white"
                title="Back to the grid (Esc)"
                @click="close"
            >
                <Icon name="heroicons:squares-2x2" size="14" />
                Grid
            </button>
            <h1 class="mr-auto flex items-center gap-1.5 text-sm font-semibold text-white">
                <Icon name="heroicons:film" size="16" class="text-lava" />
                Shorts
            </h1>

            <div class="border-border bg-panel flex rounded-lg border p-0.5 text-[11px]">
                <button
                    v-for="opt in ['newest', 'random'] as const"
                    :key="opt"
                    class="rounded-md px-2.5 py-1 font-medium capitalize transition-colors"
                    :class="sort === opt ? 'bg-lava/15 text-lava' : 'text-dim hover:text-white'"
                    @click="sort = opt"
                >
                    {{ opt }}
                </button>
            </div>

            <select
                v-model="clips"
                class="border-border bg-panel text-dim rounded-lg border px-2 py-1.5 text-[11px]
                    focus:border-white/20 focus:outline-none"
                title="Created clips"
            >
                <option v-for="opt in clipOptions" :key="opt.value" :value="opt.value">
                    {{ opt.label }}
                </option>
            </select>
        </div>

        <div v-if="ready && playing" class="min-h-0 flex-1">
            <ShortsFeed :feed="feed" @close="close" />
        </div>
        <ShortsGrid v-else-if="ready" :feed="feed" :focus-index="lastIndex" @open="open" />
    </div>
</template>
