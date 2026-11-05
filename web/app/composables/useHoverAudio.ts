const STORAGE_KEY = 'hover-audio';

const enabled = ref(true);

export function useHoverAudio() {
    function init() {
        if (import.meta.server) return;
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored !== null) {
            enabled.value = stored === 'true';
        }
    }

    function toggle() {
        enabled.value = !enabled.value;
        localStorage.setItem(STORAGE_KEY, String(enabled.value));
    }

    return {
        enabled: readonly(enabled),
        init,
        toggle,
    };
}
