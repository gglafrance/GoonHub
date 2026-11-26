/**
 * Number of public Goon2gether lobbies (badge of the Together nav link): loaded once from
 * GET /together/public-count, then kept live by the together:public_count SSE event (sent to
 * users with goon2gether:use). Started by the nav bar for those users only.
 */
export const useTogetherLobbiesStore = defineStore('togetherLobbies', () => {
    const publicCount = ref(0);

    let unsubscribe: (() => void) | null = null;

    function setCount(n: unknown) {
        if (typeof n === 'number' && Number.isFinite(n)) {
            publicCount.value = Math.max(0, Math.floor(n));
        }
    }

    async function refresh() {
        try {
            const res = await useApiTogether().getPublicCount();
            setCount(res.count);
        } catch {
            // Goon2gether disabled or offline: the badge keeps its value (0 by default)
        }
    }

    /** Starts listening to the count (idempotent). */
    function start() {
        if (unsubscribe) return;
        unsubscribe = onSSEEvent(TOGETHER_PUBLIC_COUNT_EVENT, (e) => setCount(e.data?.count));
        refresh();
    }

    function stop() {
        unsubscribe?.();
        unsubscribe = null;
        publicCount.value = 0;
    }

    return { publicCount, refresh, start, stop };
});
