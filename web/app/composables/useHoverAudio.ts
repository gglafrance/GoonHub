const STORAGE_KEY = 'hover-audio';
const VOLUME_STORAGE_KEY = 'hover-audio-volume';
const DEFAULT_VOLUME = 60;

const enabled = ref(true);
// Scene card preview sound level, 0-100
const volume = ref(DEFAULT_VOLUME);

export function useHoverAudio() {
    function init() {
        if (import.meta.server) return;
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored !== null) {
            enabled.value = stored === 'true';
        }
        const storedVolume = Number(localStorage.getItem(VOLUME_STORAGE_KEY));
        if (localStorage.getItem(VOLUME_STORAGE_KEY) !== null && !isNaN(storedVolume)) {
            volume.value = Math.min(100, Math.max(0, Math.round(storedVolume)));
        }
    }

    function toggle() {
        enabled.value = !enabled.value;
        localStorage.setItem(STORAGE_KEY, String(enabled.value));
    }

    function setVolume(value: number) {
        volume.value = Math.min(100, Math.max(0, Math.round(value)));
        localStorage.setItem(VOLUME_STORAGE_KEY, String(volume.value));
        // Raising the level of muted previews turns the sound back on
        if (volume.value > 0 && !enabled.value) {
            enabled.value = true;
            localStorage.setItem(STORAGE_KEY, 'true');
        }
    }

    return {
        enabled: readonly(enabled),
        volume: readonly(volume),
        init,
        toggle,
        setVolume,
    };
}
