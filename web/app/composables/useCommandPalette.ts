import type { PaletteItem, PaletteRecentEntry } from '~/types/command-palette';

const RECENT_STORAGE_KEY = 'command-palette-recent';
const MAX_RECENT = 8;

// Shared across the app: the header button, the global shortcut and the palette itself
const isOpen = ref(false);
// The keyboard shortcuts modal lives in the header; the palette can open it
const shortcutsVisible = ref(false);
const recent = ref<PaletteRecentEntry[]>([]);
let recentLoaded = false;

const loadRecent = () => {
    if (recentLoaded || import.meta.server) return;
    recentLoaded = true;
    try {
        const parsed = JSON.parse(localStorage.getItem(RECENT_STORAGE_KEY) || '[]');
        if (Array.isArray(parsed)) {
            recent.value = parsed
                .filter((e) => e && typeof e.id === 'string' && typeof e.label === 'string')
                .slice(0, MAX_RECENT);
        }
    } catch {
        recent.value = [];
    }
};

const saveRecent = () => {
    if (import.meta.server) return;
    try {
        localStorage.setItem(RECENT_STORAGE_KEY, JSON.stringify(recent.value));
    } catch {
        // Storage full or disabled: the list simply isn't kept
    }
};

export function useCommandPalette() {
    const open = () => {
        loadRecent();
        isOpen.value = true;
    };
    const close = () => {
        isOpen.value = false;
    };
    const toggle = () => {
        if (isOpen.value) close();
        else open();
    };

    /** Remembers an entry opened from the palette (most recent first, no duplicates). */
    const remember = (item: PaletteItem) => {
        loadRecent();
        const entry: PaletteRecentEntry = {
            id: item.id,
            group: item.group,
            label: item.label,
            sublabel: item.sublabel,
            icon: item.icon,
            image: item.image,
            imageShape: item.imageShape,
            to: item.to,
            admin: item.admin,
        };
        recent.value = [entry, ...recent.value.filter((e) => e.id !== item.id)].slice(
            0,
            MAX_RECENT,
        );
        saveRecent();
    };

    const forget = (id: string) => {
        recent.value = recent.value.filter((e) => e.id !== id);
        saveRecent();
    };

    const clearRecent = () => {
        recent.value = [];
        saveRecent();
    };

    /** Label of the modifier key of the shortcut on this platform. */
    const modifierLabel = computed(() => {
        if (import.meta.server) return 'Ctrl';
        return /Mac|iPhone|iPad/i.test(navigator.platform || navigator.userAgent) ? '⌘' : 'Ctrl';
    });

    return {
        isOpen: readonly(isOpen),
        shortcutsVisible,
        recent: readonly(recent),
        open,
        close,
        toggle,
        remember,
        forget,
        clearRecent,
        modifierLabel,
    };
}
