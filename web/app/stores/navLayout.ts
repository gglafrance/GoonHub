import type { NavLayout } from '~/types/navLayout';

const CACHE_KEY_PREFIX = 'goonhub:nav-layout:';

/**
 * Header layout of the signed-in user (docs/NAVIGATION.md), from GET /settings/navigation. The
 * last layout is cached per user in localStorage so that the header does not flicker from the
 * default layout on load; the server answer replaces it. Kept in sync across tabs and devices
 * by the settings:nav_layout SSE event. Started by the header (HeaderLiveSources).
 */
export const useNavLayoutStore = defineStore('navLayout', () => {
    /** Stored layout as the server answers it (null: not loaded and nothing cached) */
    const layout = ref<NavLayout | null>(null);
    const customized = ref(false);
    const loaded = ref(false);

    let unsubscribe: (() => void) | null = null;
    let userId: number | null = null;
    let seq = 0;

    const cacheKey = (id: number) => `${CACHE_KEY_PREFIX}${id}`;

    function readCache(id: number) {
        if (!import.meta.client) return;
        try {
            const raw = localStorage.getItem(cacheKey(id));
            if (raw) layout.value = JSON.parse(raw) as NavLayout;
        } catch {
            // a broken cache entry is ignored, the server answer replaces it
        }
    }

    function apply(next: NavLayout, isCustomized: boolean) {
        layout.value = next;
        customized.value = isCustomized;
        loaded.value = true;
        if (import.meta.client && userId !== null) {
            try {
                if (isCustomized) localStorage.setItem(cacheKey(userId), JSON.stringify(next));
                else localStorage.removeItem(cacheKey(userId));
            } catch {
                // storage full or disabled: the layout is loaded from the server each time
            }
        }
    }

    async function refresh() {
        const authStore = useAuthStore();
        if (!authStore.isAuthenticated) return;
        const mine = ++seq;
        try {
            const view = await useApiSettings().getNavLayout();
            if (mine === seq) apply(view.layout, view.customized);
        } catch {
            // the header keeps the cached or default layout
        }
    }

    /** Saves a layout; the answer is the stored layout (forbidden items left out). */
    async function save(next: NavLayout) {
        seq++;
        const view = await useApiSettings().saveNavLayout(next);
        apply(view.layout, view.customized);
    }

    /** Goes back to the default layout. */
    async function reset() {
        seq++;
        const view = await useApiSettings().resetNavLayout();
        apply(view.layout, view.customized);
    }

    /** Loads the layout of the signed-in user and follows its changes (idempotent). */
    function start() {
        if (unsubscribe || !import.meta.client) return;
        userId = useAuthStore().user?.id ?? null;
        if (userId !== null) readCache(userId);
        unsubscribe = onSSEEvent(NAV_LAYOUT_SSE_EVENT, () => refresh());
        refresh();
    }

    function stop() {
        unsubscribe?.();
        unsubscribe = null;
        userId = null;
        layout.value = null;
        customized.value = false;
        loaded.value = false;
    }

    return { layout, customized, loaded, refresh, save, reset, start, stop };
});
