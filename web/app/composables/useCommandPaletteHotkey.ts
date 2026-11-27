/**
 * Global shortcuts opening the command palette: Ctrl+K / Cmd+K anywhere (also closes it),
 * and "/" outside text fields, except on keyboard layouts where a player shortcut already
 * uses "/" (AZERTY speed up).
 */
export function useCommandPaletteHotkey() {
    const { isOpen, open, toggle } = useCommandPalette();
    const slashEnabled = useCommandPaletteSlashEnabled();

    const isTypingTarget = (target: EventTarget | null) => {
        const el = target as HTMLElement | null;
        if (!el) return false;
        return (
            el.tagName === 'INPUT' ||
            el.tagName === 'TEXTAREA' ||
            el.tagName === 'SELECT' ||
            el.isContentEditable
        );
    };

    const onKeydown = (e: KeyboardEvent) => {
        if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 'k') {
            e.preventDefault();
            toggle();
            return;
        }
        if (
            e.key === '/' &&
            slashEnabled.value &&
            !isOpen.value &&
            !e.ctrlKey &&
            !e.metaKey &&
            !e.altKey &&
            !isTypingTarget(e.target)
        ) {
            e.preventDefault();
            open();
        }
    };

    onMounted(() => window.addEventListener('keydown', onKeydown));
    onUnmounted(() => window.removeEventListener('keydown', onKeydown));
}

/** Whether "/" opens the palette with the current keyboard layout (for the shortcuts list). */
export function useCommandPaletteSlashEnabled() {
    const { keys } = useKeyboardLayout();
    return computed(() => !Object.values(keys.value).includes('/'));
}
