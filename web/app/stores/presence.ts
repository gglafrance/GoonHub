import type { PresenceView } from '~/types/presence';
import type { PresenceInputEvent } from '~/utils/presenceTracker';

/**
 * Presence of the signed-in user's contacts (friends and followed users), kept live by the
 * presence:update SSE events, and the reports of this tab: heartbeats of the real activity
 * (input, playing videos: `PresenceTracker`, online or away) and the invisible mode. Started by
 * the header (HeaderFriendsPresence).
 */
export const usePresenceStore = defineStore('presence', () => {
    // every view received, by user id (hidden ones too, to keep the most recent seq)
    const views = ref<Record<number, PresenceView>>({});
    // the users of the last snapshot (friends and followed users), in its order
    const contactIds = ref<number[]>([]);
    const invisible = ref(false);
    const loaded = ref(false);
    const heartbeatIntervalMs = ref(60_000);
    const activityIntervalMs = ref(15_000);
    const idleAfterMs = ref(300_000);
    const stalledAfterMs = ref(30_000);
    // this tab's id: each tab reports its own activity
    const tabId = newPresenceTabId();

    let unsubscribers: (() => void)[] = [];
    let stopSocialWatch: (() => void) | null = null;
    let reloadTimer: ReturnType<typeof setTimeout> | null = null;
    let tickTimer: ReturnType<typeof setInterval> | null = null;
    let stopConnectionWatch: (() => void) | null = null;
    // the activity of this tab (input, players), reported by heartbeats once started
    let started = false;
    const tracker = new PresenceTracker({
        now: () => Date.now(),
        send: (idleSeconds) => {
            if (!started || !useAuthStore().isAuthenticated) return;
            useApiPresence()
                .sendHeartbeat(true, idleSeconds)
                .catch(() => {});
        },
    });

    const contacts = computed(() =>
        contactIds.value
            .map((id) => views.value[id])
            .filter((v): v is PresenceView => !!v && v.visible),
    );
    const presentCount = computed(() => contacts.value.filter((v) => isPresent(v)).length);
    // online or watching (present but not away)
    const activeCount = computed(
        () => contacts.value.filter((v) => isPresent(v) && v.status !== 'away').length,
    );

    /** The visible presence of a user, undefined when unknown or hidden. */
    function viewOf(userId: number): PresenceView | undefined {
        const v = views.value[userId];
        return v?.visible ? v : undefined;
    }

    async function refresh() {
        const authStore = useAuthStore();
        if (!authStore.isAuthenticated) return;
        try {
            const snap = await useApiPresence().getPresence();
            for (const v of snap.users) applyPresenceView(views.value, v);
            contactIds.value = snap.users.map((v) => v.user.id);
            invisible.value = snap.invisible;
            if (snap.heartbeat_interval_seconds > 0) {
                heartbeatIntervalMs.value = snap.heartbeat_interval_seconds * 1000;
            }
            if (snap.activity_interval_seconds > 0) {
                activityIntervalMs.value = snap.activity_interval_seconds * 1000;
            }
            if (snap.idle_after_seconds > 0) idleAfterMs.value = snap.idle_after_seconds * 1000;
            if (snap.stalled_after_seconds > 0) {
                stalledAfterMs.value = snap.stalled_after_seconds * 1000;
            }
            tracker.configure({
                heartbeatMs: heartbeatIntervalMs.value,
                idleAfterMs: idleAfterMs.value,
            });
            loaded.value = true;
        } catch {
            // the list keeps its last state; the next event or reconnect refreshes it
        }
    }

    /** Reloads the snapshot soon (coalesces several social changes). */
    function scheduleRefresh() {
        if (reloadTimer) clearTimeout(reloadTimer);
        reloadTimer = setTimeout(() => {
            reloadTimer = null;
            refresh();
        }, 500);
    }

    /** Adds a view received for one user (profile page, SSE). */
    function apply(view: PresenceView) {
        applyPresenceView(views.value, view);
    }

    // a hidden tab gets no input; a tab shown again is not an input by itself
    function onInput(e: Event) {
        tracker.handleEvent(e as PresenceInputEvent, document.visibilityState === 'visible');
    }

    /**
     * Registers a player of this tab: `probe` tells whether it really plays (a playing video
     * keeps the user online). Returns the unregister function.
     */
    function registerPlayback(probe: () => boolean): () => void {
        return tracker.addPlayback(probe);
    }

    /** Starts the live presence (idempotent). */
    function start() {
        if (unsubscribers.length > 0 || !import.meta.client) return;
        unsubscribers = PRESENCE_SSE_EVENTS.map((type) =>
            onSSEEvent(type, (e) => {
                const view = e.data as unknown as PresenceView | undefined;
                if (view?.user?.id) apply(view);
            }),
        );
        // capture: an input handled (and stopped) by a component still counts
        for (const ev of PRESENCE_INPUT_EVENTS) {
            window.addEventListener(ev, onInput, { passive: true, capture: true });
        }
        started = true;
        tickTimer = setInterval(() => tracker.tick(), PRESENCE_TICK_MS);
        // the real idle time at once: at start, and when the live stream is back (server
        // restart, network loss), so that a reconnection is not taken for an activity
        tracker.sync();
        stopConnectionWatch = watch(
            () => useJobStatusStore().isConnected,
            (connected, was) => {
                if (connected && !was) tracker.sync();
            },
        );
        // friends added or removed, follows: the contacts change
        stopSocialWatch = watch(() => useSocialStore().version, scheduleRefresh);
        refresh();
    }

    function stop() {
        for (const off of unsubscribers) off();
        unsubscribers = [];
        if (import.meta.client) {
            for (const ev of PRESENCE_INPUT_EVENTS) {
                window.removeEventListener(ev, onInput, { capture: true });
            }
        }
        started = false;
        if (tickTimer) clearInterval(tickTimer);
        tickTimer = null;
        stopConnectionWatch?.();
        stopConnectionWatch = null;
        stopSocialWatch?.();
        stopSocialWatch = null;
        if (reloadTimer) clearTimeout(reloadTimer);
        reloadTimer = null;
        views.value = {};
        contactIds.value = [];
        loaded.value = false;
    }

    async function setInvisible(value: boolean) {
        const res = await useApiPresence().updatePresenceSettings({ invisible: value });
        invisible.value = res.invisible;
    }

    return {
        views,
        contactIds,
        contacts,
        presentCount,
        activeCount,
        invisible,
        loaded,
        heartbeatIntervalMs,
        activityIntervalMs,
        idleAfterMs,
        stalledAfterMs,
        tabId,
        viewOf,
        refresh,
        scheduleRefresh,
        apply,
        start,
        stop,
        registerPlayback,
        setInvisible,
    };
});
