import type videojs from 'video.js';
import type { Scene } from '~/types/scene';

type Player = ReturnType<typeof videojs>;

interface Options {
    player: ComputedRef<Player | null>;
    scene: Ref<Scene | null | undefined>;
    onTheaterModeToggle: () => void;
    onABLoopToggle?: () => void;
}

export const useScenePlayerShortcuts = (options: Options) => {
    const { player, scene, onTheaterModeToggle, onABLoopToggle } = options;
    const { keys } = useKeyboardLayout();

    const SEEK_SHORT = 5;
    const SEEK_LONG = 10;
    const VOLUME_STEP = 0.1;
    const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2];
    const DEFAULT_FRAME_RATE = 30;

    const isInputElement = (el: HTMLElement) => {
        return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable;
    };

    const handleKeydown = (e: KeyboardEvent) => {
        if (isInputElement(e.target as HTMLElement)) return;
        if (!player.value) return;

        const p = player.value;
        const key = e.key.toLowerCase();

        // Helper to get current time safely
        const getCurrentTime = () => p.currentTime() ?? 0;
        const getDuration = () => p.duration() ?? 0;
        const getVolume = () => p.volume() ?? 1;
        const getPlaybackRate = () => p.playbackRate() ?? 1;

        // Play/Pause
        if (key === ' ' || key === 'k') {
            e.preventDefault();
            if (p.paused()) {
                p.play();
            } else {
                p.pause();
            }
        }

        // Seek backward (short)
        else if (key === 'arrowleft') {
            e.preventDefault();
            p.currentTime(Math.max(0, getCurrentTime() - SEEK_SHORT));
        }
        // Seek forward (short)
        else if (key === 'arrowright') {
            e.preventDefault();
            p.currentTime(Math.min(getDuration(), getCurrentTime() + SEEK_SHORT));
        }
        // Seek backward (long)
        else if (key === 'j') {
            e.preventDefault();
            p.currentTime(Math.max(0, getCurrentTime() - SEEK_LONG));
        }
        // Seek forward (long)
        else if (key === 'l') {
            e.preventDefault();
            p.currentTime(Math.min(getDuration(), getCurrentTime() + SEEK_LONG));
        }

        // Volume up
        else if (key === 'arrowup' || key === '=' || key === '+') {
            e.preventDefault();
            p.volume(Math.min(1, getVolume() + VOLUME_STEP));
        }
        // Volume down
        else if (key === 'arrowdown' || key === '-') {
            e.preventDefault();
            p.volume(Math.max(0, getVolume() - VOLUME_STEP));
        }
        // Mute toggle (N, not M - M is used for markers)
        else if (key === 'n') {
            e.preventDefault();
            p.muted(!p.muted());
        }

        // Fullscreen toggle
        else if (key === 'f') {
            e.preventDefault();
            if (p.isFullscreen()) {
                p.exitFullscreen();
            } else {
                p.requestFullscreen();
            }
        }

        // Decrease playback speed
        else if (
            key === keys.value.speedDecrease ||
            (key === keys.value.speedDecreaseShift && e.shiftKey)
        ) {
            e.preventDefault();
            const currentRate = getPlaybackRate();
            const idx = PLAYBACK_RATES.indexOf(currentRate);
            if (idx > 0) {
                p.playbackRate(PLAYBACK_RATES[idx - 1]);
            } else if (idx === -1) {
                // Current rate not in list, find closest lower rate
                const lowerRate = PLAYBACK_RATES.filter((r) => r < currentRate).pop();
                if (lowerRate) p.playbackRate(lowerRate);
            }
        }
        // Increase playback speed
        else if (
            key === keys.value.speedIncrease ||
            (key === keys.value.speedIncreaseShift && e.shiftKey)
        ) {
            e.preventDefault();
            const currentRate = getPlaybackRate();
            const idx = PLAYBACK_RATES.indexOf(currentRate);
            if (idx >= 0 && idx < PLAYBACK_RATES.length - 1) {
                p.playbackRate(PLAYBACK_RATES[idx + 1]);
            } else if (idx === -1) {
                // Current rate not in list, find closest higher rate
                const higherRate = PLAYBACK_RATES.find((r) => r > currentRate);
                if (higherRate) p.playbackRate(higherRate);
            }
        }

        // Frame step backward (only when paused)
        else if (key === keys.value.frameBack && !e.shiftKey && p.paused()) {
            e.preventDefault();
            const frameRate = scene.value?.frame_rate ?? DEFAULT_FRAME_RATE;
            p.currentTime(Math.max(0, getCurrentTime() - 1 / frameRate));
        }
        // Frame step forward (only when paused)
        else if (key === keys.value.frameForward && !e.shiftKey && p.paused()) {
            e.preventDefault();
            const frameRate = scene.value?.frame_rate ?? DEFAULT_FRAME_RATE;
            p.currentTime(Math.min(getDuration(), getCurrentTime() + 1 / frameRate));
        }

        // Theater mode toggle
        else if (key === 't') {
            e.preventDefault();
            onTheaterModeToggle();
        }

        // AB loop toggle
        else if (key === 'o') {
            e.preventDefault();
            if (onABLoopToggle) onABLoopToggle();
        }

        // Picture-in-Picture toggle
        else if (key === 'p') {
            e.preventDefault();
            if (p.isInPictureInPicture?.()) {
                p.exitPictureInPicture?.();
            } else {
                p.requestPictureInPicture?.();
            }
        }
    };

    onMounted(() => {
        window.addEventListener('keydown', handleKeydown);
    });

    onUnmounted(() => {
        window.removeEventListener('keydown', handleKeydown);
    });
};
