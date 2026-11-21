import type { NavEntries } from '~/utils/navHistory';

const STORAGE_KEY = 'nav-history-entries';

// Mirror of the tab history, kept in sessionStorage so that it survives a reload
const entries = ref<NavEntries>({});
let loaded = false;

function load() {
    if (loaded || !import.meta.client) return;
    loaded = true;
    try {
        entries.value = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || '{}');
    } catch {
        entries.value = {};
    }
}

/** Current history position stored by vue-router (null outside the router's entries). */
function currentPosition(): number | null {
    const pos = window.history.state?.position;
    return typeof pos === 'number' ? pos : null;
}

/** Records the current entry (called after each navigation, see plugins/nav-history.client.ts). */
export function recordCurrentNavEntry(fullPath: string) {
    load();
    const pos = currentPosition();
    if (pos === null) return;
    entries.value = recordNavEntry(entries.value, pos, fullPath);
    try {
        sessionStorage.setItem(STORAGE_KEY, JSON.stringify(entries.value));
    } catch {
        // Storage full or disabled: the origin is only kept for this page load
    }
}

/**
 * The list the user came from (closest earlier history entry that is not a scene page), and a
 * goBack() that returns to it through the history, so that the list comes back with its
 * filters, page and scroll position. Without an origin, goBack() opens the fallback.
 */
export function useNavOrigin() {
    const router = useRouter();
    const route = useRoute();
    load();

    const origin = computed(() => {
        // Recomputed on each navigation of the page (related scenes, playlist steps)
        void route.fullPath;
        void entries.value;
        if (!import.meta.client) return null;
        const pos = currentPosition();
        return pos === null ? null : findNavOrigin(entries.value, pos);
    });

    const goBack = (fallback: string) => {
        const pos = currentPosition();
        const target = origin.value;
        if (target && pos !== null) {
            router.go(target.position - pos);
        } else {
            router.push(fallback);
        }
    };

    /** Back to a list page: through the history when it is the previous entry, else opened. */
    const backTo = (path: string) => {
        const pos = currentPosition();
        if (pos !== null && previousEntryIs(entries.value, pos, path)) router.back();
        else router.push(path);
    };

    return { origin, goBack, backTo };
}
