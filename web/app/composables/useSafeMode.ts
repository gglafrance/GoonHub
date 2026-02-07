const STORAGE_KEY = 'safe-mode';

const enabled = ref(false);

function apply(value: boolean) {
    if (import.meta.server) return;
    document.documentElement.classList.toggle('safe-mode', value);
}

export function useSafeMode() {
    function init() {
        if (import.meta.server) return;
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored === 'true') {
            enabled.value = true;
            apply(true);
        }
    }

    function toggle() {
        enabled.value = !enabled.value;
        localStorage.setItem(STORAGE_KEY, String(enabled.value));
        apply(enabled.value);
    }

    return {
        enabled: readonly(enabled),
        init,
        toggle,
    };
}
