import type { NavItemDef } from '~/utils/navRegistry';

export interface HeaderItemBadge {
    text: string;
    title: string;
}

/**
 * What the header items need wherever they are placed (docs/NAVIGATION.md): the live badges
 * (public lobbies, unread messages, friend requests), the page of an item shown as a menu row
 * in a folder, and the actions of the items that are not pages (palette, shortcuts, safe mode,
 * preview sound, logout).
 */
export function useHeaderItems() {
    const authStore = useAuthStore();
    const socialStore = useSocialStore();
    const messagesStore = useMessagesStore();
    const togetherLobbies = useTogetherLobbiesStore();
    const { open: openPalette, shortcutsVisible } = useCommandPalette();
    const { enabled: safeModeEnabled, toggle: toggleSafeMode } = useSafeMode();
    const { enabled: soundEnabled, volume, toggle: toggleSound } = useHoverAudio();

    function badge(id: string): HeaderItemBadge | null {
        switch (id) {
            case 'together': {
                const n = togetherLobbies.publicCount;
                const text = badgeCount(n);
                return text ? { text, title: `${n} public lobb${n === 1 ? 'y' : 'ies'}` } : null;
            }
            case 'messages': {
                const n = messagesStore.unread.messages;
                const text = unreadBadge(n);
                return text ? { text, title: `${n} unread` } : null;
            }
            case 'friends': {
                const n = socialStore.incomingRequests;
                const text = badgeCount(n);
                return text ? { text, title: `${n} pending request${n > 1 ? 's' : ''}` } : null;
            }
            default:
                return null;
        }
    }

    /** Page of an item in a folder menu, null for the action items. */
    function route(item: NavItemDef): string | null {
        if (item.id === 'profile') {
            return authStore.user ? profilePath(authStore.user.username) : null;
        }
        return item.to ?? null;
    }

    /** Label and icon of a menu row (the toggles show their state). */
    function menuRow(item: NavItemDef): { label: string; icon: string } {
        switch (item.id) {
            case 'safe_mode':
                return safeModeEnabled.value
                    ? { label: 'Disable safe mode', icon: 'heroicons:eye-slash' }
                    : { label: 'Enable safe mode', icon: 'heroicons:eye' };
            case 'preview_sound':
                return soundEnabled.value && volume.value > 0
                    ? { label: 'Mute preview sound', icon: 'heroicons:speaker-wave' }
                    : { label: 'Play preview sound', icon: 'heroicons:speaker-x-mark' };
            default:
                return { label: item.label, icon: item.icon };
        }
    }

    /** Runs the action of an item that is not a page. */
    function run(item: NavItemDef) {
        switch (item.id) {
            case 'command_palette':
                openPalette();
                break;
            case 'shortcuts':
                shortcutsVisible.value = true;
                break;
            case 'safe_mode':
                toggleSafeMode();
                break;
            case 'preview_sound':
                toggleSound();
                break;
            case 'logout':
                authStore.logout();
                break;
        }
    }

    return { badge, route, menuRow, run };
}
