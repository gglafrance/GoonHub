import type { ClipsFilter, ShortItem, ShortsSort } from '~/types/shorts';

const PAGE_SIZE = 50;
// Fetch the next page when the active short is this close to the end.
const PREFETCH_DISTANCE = 3;
// One wheel gesture moves one short. Events count as a new gesture after a
// pause, or when the wheel speeds up again (a new flick during trackpad
// momentum). The rest of a gesture is ignored, but never for longer than
// WHEEL_MAX_LOCK_MS, so steady scrolling keeps moving.
const WHEEL_MIN_DELTA = 8;
const WHEEL_GESTURE_GAP_MS = 180;
const WHEEL_MIN_INTERVAL_MS = 300;
const WHEEL_MAX_LOCK_MS = 1200;

/**
 * Paging, the active slide and navigation for the Shorts page. The grid and
 * the player share one list. The player's container is a vertical
 * snap-scroller with one slide per short; it is null while the grid shows.
 */
export const useShortsFeed = (sort: Ref<ShortsSort>, clips: Ref<ClipsFilter>) => {
    const { getShorts } = useApiShorts();

    const container = ref<HTMLElement | null>(null);

    const items = ref<ShortItem[]>([]);
    const total = ref(0);
    const page = ref(0);
    const seed = ref<number | undefined>(undefined);
    const maxDuration = ref(60);
    const loading = ref(false);
    const error = ref('');
    const done = ref(false);
    const activeIndex = ref(0);

    const likes = ref<Record<string, boolean>>({});
    const ratings = ref<Record<string, number>>({});
    const jizzCounts = ref<Record<string, number>>({});

    let generation = 0;

    const loadMore = async () => {
        if (loading.value || done.value) return;
        loading.value = true;
        error.value = '';
        const gen = generation;
        try {
            const res = await getShorts({
                sort: sort.value,
                page: page.value + 1,
                limit: PAGE_SIZE,
                seed: sort.value === 'random' ? seed.value : undefined,
                clips: clips.value,
            });
            if (gen !== generation) return;
            page.value += 1;
            items.value = [...items.value, ...res.data];
            total.value = res.total;
            maxDuration.value = res.max_duration;
            if (res.seed) seed.value = res.seed;
            likes.value = { ...likes.value, ...(res.likes ?? {}) };
            ratings.value = { ...ratings.value, ...(res.ratings ?? {}) };
            jizzCounts.value = { ...jizzCounts.value, ...(res.jizz_counts ?? {}) };
            if (res.data.length < PAGE_SIZE || items.value.length >= res.total) done.value = true;
        } catch (e: unknown) {
            if (gen === generation)
                error.value = e instanceof Error ? e.message : 'Failed to load shorts';
        } finally {
            if (gen === generation) loading.value = false;
        }
    };

    const reset = async () => {
        generation++;
        items.value = [];
        total.value = 0;
        page.value = 0;
        seed.value = undefined;
        done.value = false;
        loading.value = false;
        activeIndex.value = 0;
        targetIndex = null;
        likes.value = {};
        ratings.value = {};
        jizzCounts.value = {};
        container.value?.scrollTo({ top: 0 });
        await loadMore();
    };

    watch(activeIndex, (i) => {
        if (i >= items.value.length - PREFETCH_DISTANCE) loadMore();
    });

    // The active slide is the one at least 60% in view. Slides can register
    // before the container is set, so they are kept and observed once it is.
    let observer: IntersectionObserver | null = null;
    const slidesSeen = new Set<Element>();
    const observe = (el: Element | null) => {
        if (!el) return;
        slidesSeen.add(el);
        observer?.observe(el);
    };

    watch(container, (el) => {
        observer?.disconnect();
        observer = null;
        if (!el) {
            slidesSeen.clear();
            return;
        }
        observer = new IntersectionObserver(
            (entries) => {
                for (const entry of entries) {
                    if (!entry.isIntersecting) continue;
                    const index = Number((entry.target as HTMLElement).dataset.index);
                    if (!Number.isNaN(index)) activeIndex.value = index;
                }
            },
            { root: el, threshold: 0.6 },
        );
        for (const slide of slidesSeen) {
            if (slide.isConnected) observer.observe(slide);
            else slidesSeen.delete(slide);
        }
    });

    onBeforeUnmount(() => {
        clearTimeout(targetTimer);
        observer?.disconnect();
        observer = null;
    });

    // Where a smooth scroll is heading, so a second press during it moves on
    // from there rather than from the slide still on screen.
    let targetIndex: number | null = null;
    let targetTimer: ReturnType<typeof setTimeout> | undefined;

    const scrollToIndex = (index: number) => {
        const el = container.value;
        if (!el || index < 0 || index >= items.value.length) return;
        const slide = el.querySelector<HTMLElement>(`[data-index="${index}"]`);
        if (!slide) return;
        targetIndex = index;
        clearTimeout(targetTimer);
        targetTimer = setTimeout(() => (targetIndex = null), 1000);
        slide.scrollIntoView({ behavior: 'smooth', block: 'start' });
    };

    watch(activeIndex, (i) => {
        if (i === targetIndex) targetIndex = null;
    });

    // Show a slide straight away, without the smooth scroll (opening from the grid).
    const jumpTo = (index: number) => {
        if (index < 0 || index >= items.value.length) return;
        activeIndex.value = index;
        targetIndex = null;
        nextTick(() => {
            container.value
                ?.querySelector<HTMLElement>(`[data-index="${index}"]`)
                ?.scrollIntoView({ block: 'start' });
        });
    };

    const next = () => scrollToIndex((targetIndex ?? activeIndex.value) + 1);
    const prev = () => scrollToIndex((targetIndex ?? activeIndex.value) - 1);

    let lastWheelAt = 0;
    let lastWheelDelta = 0;
    let lastMoveAt = 0;
    const onWheel = (e: WheelEvent) => {
        e.preventDefault();
        const delta = Math.abs(e.deltaY);
        if (delta < WHEEL_MIN_DELTA) return;
        const now = Date.now();
        const newGesture =
            now - lastWheelAt > WHEEL_GESTURE_GAP_MS || delta > lastWheelDelta * 1.5 + 4;
        lastWheelAt = now;
        lastWheelDelta = delta;
        const sinceMove = now - lastMoveAt;
        if (sinceMove < WHEEL_MIN_INTERVAL_MS) return;
        if (!newGesture && sinceMove < WHEEL_MAX_LOCK_MS) return;
        lastMoveAt = now;
        if (e.deltaY > 0) next();
        else prev();
    };

    return {
        container,
        items,
        total,
        seed,
        maxDuration,
        loading,
        error,
        done,
        activeIndex,
        likes,
        ratings,
        jizzCounts,
        loadMore,
        reset,
        observe,
        jumpTo,
        next,
        prev,
        onWheel,
    };
};

export type ShortsFeed = ReturnType<typeof useShortsFeed>;
