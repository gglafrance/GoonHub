import type { PresenceView } from '~/types/presence';

// Input events that count as activity (the tab is used): a heartbeat is sent at most every
// heartbeat interval, so the frequent ones (mousemove) cost nothing more.
const INPUT_EVENTS = ['pointerdown', 'keydown', 'wheel', 'touchstart', 'mousemove'] as const;

/**
 * Presence of the signed-in user's contacts (friends and followed users), kept live by the
 * presence:update SSE events, and the reports of this tab: input heartbeats (not idle) and the
 * invisible mode. Started by the header (HeaderFriendsPresence).
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
    // this tab's id: each tab reports its own activity
    const tabId = newPresenceTabId();

    let unsubscribers: (() => void)[] = [];
    let stopSocialWatch: (() => void) | null = null;
    let reloadTimer: ReturnType<typeof setTimeout> | null = null;
    let lastHeartbeat = 0;

    const contacts = computed(() =>
        contactIds.value
            .map((id) => views.value[id])
            .filter((v): v is PresenceView => !!v && v.visible),
    );
    const presentCount = computed(() => contacts.value.filter((v) => isPresent(v)).length);

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

    function onInput() {
        const now = Date.now();
        if (now - lastHeartbeat < heartbeatIntervalMs.value) return;
        lastHeartbeat = now;
        useApiPresence()
            .sendHeartbeat(true)
            .catch(() => {});
    }

    function onVisibility() {
        if (document.visibilityState === 'visible') onInput();
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
        for (const ev of INPUT_EVENTS) {
            window.addEventListener(ev, onInput, { passive: true });
        }
        document.addEventListener('visibilitychange', onVisibility);
        // friends added or removed, follows: the contacts change
        stopSocialWatch = watch(() => useSocialStore().version, scheduleRefresh);
        refresh();
    }

    function stop() {
        for (const off of unsubscribers) off();
        unsubscribers = [];
        if (import.meta.client) {
            for (const ev of INPUT_EVENTS) window.removeEventListener(ev, onInput);
            document.removeEventListener('visibilitychange', onVisibility);
        }
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
        invisible,
        loaded,
        heartbeatIntervalMs,
        activityIntervalMs,
        tabId,
        viewOf,
        refresh,
        scheduleRefresh,
        apply,
        start,
        stop,
        setInvisible,
    };
});
