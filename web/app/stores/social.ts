import type { SocialEventData } from '~/types/social';

/**
 * Social state of the signed-in user: the pending incoming friend requests count (header
 * badge), kept live by the social SSE events. `version` bumps on every social event so that
 * open lists (the /friends page, a profile) can reload.
 */
export const useSocialStore = defineStore('social', () => {
    const incomingRequests = ref(0);
    const loaded = ref(false);
    const version = ref(0);

    let unsubscribers: (() => void)[] = [];

    async function refresh() {
        const authStore = useAuthStore();
        if (!authStore.isAuthenticated) return;
        try {
            const summary = await useApiSocial().getSummary();
            incomingRequests.value = summary.incoming_requests;
            loaded.value = true;
        } catch {
            // the badge keeps its last value; the next event or reconnect refreshes it
        }
    }

    function setIncoming(n: number) {
        incomingRequests.value = Math.max(0, n);
    }

    function onEvent(data: SocialEventData | undefined) {
        if (typeof data?.incoming_requests === 'number') {
            setIncoming(data.incoming_requests);
        }
        version.value++;
    }

    /** Starts listening to the social SSE events (idempotent). */
    function start() {
        if (unsubscribers.length > 0) return;
        unsubscribers = SOCIAL_SSE_EVENTS.map((type) =>
            onSSEEvent(type, (e) => onEvent(e.data as SocialEventData | undefined)),
        );
        refresh();
    }

    function stop() {
        for (const off of unsubscribers) off();
        unsubscribers = [];
        incomingRequests.value = 0;
        loaded.value = false;
    }

    /** Called after an action of this tab changed the incoming requests. */
    function changed() {
        version.value++;
        refresh();
    }

    return { incomingRequests, loaded, version, refresh, start, stop, setIncoming, changed };
});
