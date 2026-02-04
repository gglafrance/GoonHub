export interface KeyboardShortcut {
    key: string;
    ctrl?: boolean;
    shift?: boolean;
    alt?: boolean;
    action: () => void;
    description?: string;
}

export const useKeyboardShortcuts = (
    shortcuts: KeyboardShortcut[],
    enabled: Ref<boolean> = ref(true),
) => {
    const handleKeydown = (e: KeyboardEvent) => {
        if (!enabled.value) return;

        // Skip if focused on input/textarea/contenteditable
        const target = e.target as HTMLElement;
        if (
            target.tagName === 'INPUT' ||
            target.tagName === 'TEXTAREA' ||
            target.isContentEditable
        ) {
            return;
        }

        for (const shortcut of shortcuts) {
            const ctrlMatch = shortcut.ctrl ? e.ctrlKey || e.metaKey : !(e.ctrlKey || e.metaKey);
            const shiftMatch = shortcut.shift ? e.shiftKey : !e.shiftKey;
            const altMatch = shortcut.alt ? e.altKey : !e.altKey;
            const keyMatch = e.key.toLowerCase() === shortcut.key.toLowerCase();

            if (ctrlMatch && shiftMatch && altMatch && keyMatch) {
                e.preventDefault();
                shortcut.action();
                return;
            }
        }
    };

    onMounted(() => {
        window.addEventListener('keydown', handleKeydown);
    });

    onUnmounted(() => {
        window.removeEventListener('keydown', handleKeydown);
    });

    return {
        shortcuts,
    };
};
