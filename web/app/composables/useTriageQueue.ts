/**
 * Triage queue: the ordered list of scene IDs matching a search, walked one scene at a time.
 *
 * The queue is built from the regular search endpoint, one page at a time. Scenes already
 * seen stay in the list (so Previous works and the position is stable), and scenes that leave
 * the search because of a triage action (tagged in a "No tags" queue, trashed...) do not make
 * the queue skip anything: the next page is fetched from a little before the expected
 * offset, and the IDs already in the list are dropped. The state is kept in sessionStorage so
 * the exact position survives a trip to another page.
 */
import type { SceneListItem } from '~/types/scene';

export type TriageParams = Record<string, string | number | undefined>;

const PAGE_SIZE = 50;
// Touched scenes can still be in the search index for a moment (indexing is async)
const RECENT_TOUCH_MS = 15_000;
// Pages fetched in a row while looking for unseen scenes
const MAX_PAGES_PER_LOAD = 4;
// Load more when the user gets this close to the end of the loaded list
const PREFETCH_MARGIN = 5;
const STORAGE_KEY = 'goonhub:triage-queue';

interface StoredQueue {
    key: string;
    base: number;
    ids: number[];
    serverNext: number;
    serverTotal: number;
    done: boolean;
    fetchedAt: number;
    touched: Record<string, number>;
}

interface PageResult {
    data: SceneListItem[];
    total: number;
}

/** Stable signature of search params (the queue identity). */
export const triageQueueKey = (params: TriageParams): string => {
    const entries = Object.entries(params)
        .filter(([, v]) => v !== undefined && v !== '' && v !== 0)
        .sort(([a], [b]) => a.localeCompare(b));
    return JSON.stringify(entries);
};

export function useTriageQueue() {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const key = ref('');
    let params: TriageParams = {};

    const ids = ref<number[]>([]);
    // Server index of ids[0] (non-zero when the queue was opened from a URL without state)
    const base = ref(0);
    const index = ref(0);
    const serverNext = ref(0);
    const serverTotal = ref(0);
    const done = ref(false);
    const loading = ref(false);
    const error = ref('');
    // Scenes skipped by navigation (moved to trash)
    const skipped = ref(new Set<number>());

    let fetchedAt = 0;
    let touched = new Map<number, number>();
    let generation = 0;
    let loadingMore: Promise<void> | null = null;

    const currentId = computed<number | null>(() => ids.value[index.value] ?? null);
    const position = computed(() => (ids.value.length ? base.value + index.value + 1 : 0));
    const total = computed(
        () =>
            base.value +
            ids.value.length +
            (done.value ? 0 : Math.max(0, serverTotal.value - serverNext.value)),
    );
    const isEmpty = computed(() => !loading.value && done.value && ids.value.length === 0);

    const fetchPage = async (page: number): Promise<PageResult> => {
        const query = new URLSearchParams();
        for (const [k, v] of Object.entries(params)) {
            if (v !== undefined && v !== '' && v !== 0) query.set(k, String(v));
        }
        query.set('page', String(page));
        query.set('limit', String(PAGE_SIZE));
        const response = await fetch(`/api/v1/scenes?${query}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const result = await handleResponse(response);
        return { data: result.data || [], total: result.total || 0 };
    };

    const persist = () => {
        if (import.meta.server || !key.value) return;
        const stored: StoredQueue = {
            key: key.value,
            base: base.value,
            ids: ids.value,
            serverNext: serverNext.value,
            serverTotal: serverTotal.value,
            done: done.value,
            fetchedAt,
            touched: Object.fromEntries(touched),
        };
        try {
            sessionStorage.setItem(STORAGE_KEY, JSON.stringify(stored));
        } catch {
            // Storage full or unavailable: the queue still works, without restore
        }
    };

    const readStored = (): StoredQueue | null => {
        if (import.meta.server) return null;
        try {
            const raw = sessionStorage.getItem(STORAGE_KEY);
            return raw ? (JSON.parse(raw) as StoredQueue) : null;
        } catch {
            return null;
        }
    };

    const resetState = (newParams: TriageParams) => {
        generation++;
        params = { ...newParams };
        key.value = triageQueueKey(params);
        ids.value = [];
        base.value = 0;
        index.value = 0;
        serverNext.value = 0;
        serverTotal.value = 0;
        done.value = false;
        error.value = '';
        skipped.value = new Set();
        fetchedAt = 0;
        touched = new Map();
        loadingMore = null;
    };

    /** Appends the next unseen scenes of the search to the list. */
    const loadMore = (): Promise<void> => {
        if (done.value) return Promise.resolve();
        if (loadingMore) return loadingMore;
        const gen = generation;
        loadingMore = (async () => {
            const startedAt = Date.now();
            const seen = new Set(ids.value);
            let recent = 0;
            for (const t of touched.values()) {
                if (t >= fetchedAt - RECENT_TOUCH_MS) recent++;
            }
            // Scenes that left the search shift the rest towards the start: begin early
            const start = Math.max(0, serverNext.value - recent);
            let page = Math.floor(start / PAGE_SIZE) + 1;
            let added = 0;
            try {
                for (let i = 0; i < MAX_PAGES_PER_LOAD && added === 0; i++, page++) {
                    const res = await fetchPage(page);
                    if (gen !== generation) return;
                    const fresh: number[] = [];
                    for (const scene of res.data) {
                        if (!seen.has(scene.id)) {
                            seen.add(scene.id);
                            fresh.push(scene.id);
                        }
                    }
                    ids.value.push(...fresh);
                    added += fresh.length;
                    serverTotal.value = res.total;
                    serverNext.value = (page - 1) * PAGE_SIZE + res.data.length;
                    if (res.data.length < PAGE_SIZE || serverNext.value >= res.total) {
                        done.value = true;
                        break;
                    }
                }
                fetchedAt = startedAt;
                error.value = '';
                persist();
            } catch (e: unknown) {
                if (gen === generation) {
                    error.value = e instanceof Error ? e.message : 'Failed to load the queue';
                }
            }
        })().finally(() => {
            if (gen === generation) loadingMore = null;
        });
        return loadingMore;
    };

    /** Prepends the page before ids[0] (queue opened mid-way without stored state). */
    const loadBefore = async () => {
        if (base.value <= 0) return;
        const gen = generation;
        const page = Math.ceil(base.value / PAGE_SIZE);
        let res: PageResult;
        try {
            res = await fetchPage(page);
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load the queue';
            return;
        }
        if (gen !== generation) return;
        const seen = new Set(ids.value);
        const fresh = res.data.map((s) => s.id).filter((id) => !seen.has(id));
        ids.value.unshift(...fresh);
        index.value += fresh.length;
        base.value = page <= 1 ? 0 : Math.max(0, base.value - fresh.length);
        persist();
    };

    const prefetchIfNeeded = () => {
        if (!done.value && ids.value.length - index.value <= PREFETCH_MARGIN) {
            void loadMore();
        }
    };

    /**
     * Opens the queue for a search. Restores the stored state when it is the same search,
     * otherwise starts at the given 1-based position (and at sceneId when found there).
     */
    const open = async (newParams: TriageParams, at: number, sceneId: number | null) => {
        resetState(newParams);
        const gen = generation;
        loading.value = true;
        try {
            const stored = readStored();
            if (stored && stored.key === key.value && stored.ids.length > 0) {
                ids.value = stored.ids;
                base.value = stored.base;
                serverNext.value = stored.serverNext;
                serverTotal.value = stored.serverTotal;
                done.value = stored.done;
                fetchedAt = stored.fetchedAt;
                touched = new Map(
                    Object.entries(stored.touched || {}).map(([k, v]) => [Number(k), v]),
                );
                const found = sceneId !== null ? ids.value.indexOf(sceneId) : -1;
                index.value =
                    found >= 0
                        ? found
                        : Math.min(Math.max(0, at - 1 - base.value), ids.value.length - 1);
                prefetchIfNeeded();
                return;
            }

            if (at > 1) {
                // No stored state: open the page holding the position
                const page = Math.floor((at - 1) / PAGE_SIZE) + 1;
                const res = await fetchPage(page);
                if (gen !== generation) return;
                if (res.data.length > 0) {
                    base.value = (page - 1) * PAGE_SIZE;
                    ids.value = res.data.map((s) => s.id);
                    serverNext.value = base.value + res.data.length;
                    serverTotal.value = res.total;
                    done.value = res.data.length < PAGE_SIZE || serverNext.value >= res.total;
                    fetchedAt = Date.now();
                    const found = sceneId !== null ? ids.value.indexOf(sceneId) : -1;
                    index.value =
                        found >= 0
                            ? found
                            : Math.min(Math.max(0, at - 1 - base.value), ids.value.length - 1);
                    persist();
                    prefetchIfNeeded();
                    return;
                }
                // The queue got shorter than the position: start over
            }

            await loadMore();
            if (gen !== generation) return;
            const found = sceneId !== null ? ids.value.indexOf(sceneId) : -1;
            index.value = Math.max(0, found);
            prefetchIfNeeded();
        } catch (e: unknown) {
            if (gen === generation) {
                error.value = e instanceof Error ? e.message : 'Failed to load the queue';
            }
        } finally {
            if (gen === generation) loading.value = false;
        }
    };

    /** Moves to the next scene not skipped. Returns false at the end of the queue. */
    const next = async (): Promise<boolean> => {
        const gen = generation;
        let target = index.value + 1;
        for (;;) {
            while (target < ids.value.length && skipped.value.has(ids.value[target]!)) target++;
            if (target < ids.value.length) break;
            if (done.value) return false;
            const before = ids.value.length;
            await loadMore();
            if (gen !== generation) return false;
            if (ids.value.length === before && (done.value || error.value)) return false;
        }
        index.value = target;
        prefetchIfNeeded();
        return true;
    };

    /** Moves to the previous scene not skipped. Returns false at the start of the queue. */
    const previous = async (): Promise<boolean> => {
        const gen = generation;
        let target = index.value - 1;
        for (;;) {
            while (target >= 0 && skipped.value.has(ids.value[target]!)) target--;
            if (target >= 0) break;
            if (base.value <= 0) return false;
            const before = ids.value.length;
            await loadBefore();
            if (gen !== generation) return false;
            const added = ids.value.length - before;
            if (added === 0) return false;
            target += added;
        }
        index.value = target;
        return true;
    };

    /** Peeks the IDs following the current one (for detail prefetching). */
    const upcoming = (count: number): number[] =>
        ids.value
            .slice(index.value + 1)
            .filter((id) => !skipped.value.has(id))
            .slice(0, count);

    /** Records an action on a scene that may make it leave the search. */
    const touch = (id: number) => {
        touched.set(id, Date.now());
        persist();
    };

    const skip = (id: number) => {
        skipped.value = new Set(skipped.value).add(id);
        touch(id);
    };

    const unskip = (id: number) => {
        const nextSet = new Set(skipped.value);
        nextSet.delete(id);
        skipped.value = nextSet;
    };

    /** Jumps back to a scene still in the list (e.g. after undoing a trash). */
    const goTo = (id: number) => {
        const i = ids.value.indexOf(id);
        if (i >= 0) index.value = i;
    };

    return {
        key,
        ids: readonly(ids),
        index: readonly(index),
        currentId,
        position,
        total,
        done: readonly(done),
        loading: readonly(loading),
        error: readonly(error),
        isEmpty,
        skipped: readonly(skipped),
        open,
        next,
        previous,
        upcoming,
        touch,
        skip,
        unskip,
        goTo,
    };
}
