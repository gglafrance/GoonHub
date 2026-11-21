import type videojs from 'video.js';
import type { VttCue } from './useVttParser';
import type {
    GestureAxis,
    GestureSeekStreak,
    GestureSide,
    GestureTapZone,
} from '~/utils/playerGestures';

type Player = ReturnType<typeof videojs>;

export type PlayerGestureIndicator =
    | { kind: 'seek'; side: GestureSide; amount: number; id: number }
    | { kind: 'volume'; level: number }
    | { kind: 'brightness'; level: number }
    | { kind: 'scrub'; time: number; delta: number; duration: number; cue: VttCue | null }
    | { kind: 'speed'; rate: number };

interface Options {
    player: Ref<Player | null>;
    vttCues: Ref<VttCue[]>;
    enabled: Ref<boolean>;
    seekStep: Ref<number>;
}

type TouchMode = 'pending' | 'scrub' | 'volume' | 'brightness' | 'speed' | 'ignore';

interface TouchState {
    id: number;
    startX: number;
    startY: number;
    startAt: number;
    rect: DOMRect;
    mode: TouchMode;
    startTime: number;
    startLevel: number;
    targetTime: number;
    prevRate: number;
}

// Elements of the player that keep their own touch handling
const CONTROL_SELECTOR =
    '.vjs-control-bar, .vjs-menu, .vjs-modal-dialog, .vjs-big-play-button, button, a, input';

// iOS and iPadOS do not let scripts set the volume (it stays at the hardware level)
function isAppleTouchDevice(): boolean {
    return (
        /iPad|iPhone|iPod/.test(navigator.userAgent) ||
        (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
    );
}

/**
 * Touch gestures of the scene player (touch input only, mouse and keyboard are untouched):
 * - double-tap on the left/right third: seek back/forward by the seek step, repeated taps add up
 * - horizontal swipe: scrub, previewed with the sprite thumbnails, applied on release
 * - vertical swipe in fullscreen: volume on the right half (when the browser allows it),
 *   brightness on the left half (CSS filter on the video); inline, it scrolls the page
 * - long-press: 2x speed while held
 * A single tap keeps the video.js behaviour (toggling the controls). Events handled by a gesture
 * are stopped in the capture phase, before video.js sees them.
 */
export function usePlayerGestures(options: Options) {
    const { player, vttCues, enabled, seekStep } = options;

    const indicator = shallowRef<PlayerGestureIndicator | null>(null);
    const brightness = ref(1);

    let rootEl: HTMLElement | null = null;
    let touch: TouchState | null = null;
    let lastTap: { at: number; zone: GestureTapZone } | null = null;
    let streak: GestureSeekStreak | null = null;
    let lastTouchAt = 0;
    let seekId = 0;
    let longPressTimer: ReturnType<typeof setTimeout> | null = null;
    let hideTimer: ReturnType<typeof setTimeout> | null = null;
    const stopWatchers: (() => void)[] = [];

    function show(value: PlayerGestureIndicator) {
        if (hideTimer) {
            clearTimeout(hideTimer);
            hideTimer = null;
        }
        indicator.value = value;
    }

    function hideSoon(delay: number) {
        if (hideTimer) clearTimeout(hideTimer);
        hideTimer = setTimeout(() => {
            indicator.value = null;
            hideTimer = null;
        }, delay);
    }

    function clearLongPress() {
        if (longPressTimer) {
            clearTimeout(longPressTimer);
            longPressTimer = null;
        }
    }

    function techEl(): HTMLVideoElement | null {
        return (rootEl?.querySelector('video.vjs-tech') as HTMLVideoElement | null) ?? null;
    }

    function canSetVolume(p: Player): boolean {
        const tech = (p as unknown as { tech?: (safety: boolean) => unknown }).tech?.(true) as
            | { featuresVolumeControl?: boolean }
            | undefined;
        if (tech?.featuresVolumeControl === false) return false;
        return !isAppleTouchDevice();
    }

    function isGestureTarget(target: EventTarget | null): boolean {
        if (!(target instanceof Element) || !rootEl || !rootEl.contains(target)) return false;
        return !target.closest(CONTROL_SELECTOR);
    }

    function findTouch(list: TouchList, id: number): Touch | null {
        for (let i = 0; i < list.length; i++) {
            const t = list.item(i);
            if (t && t.identifier === id) return t;
        }
        return null;
    }

    // Stops an event handled by a gesture: video.js must not see it (no tap, no click)
    function consume(e: Event) {
        if (e.cancelable) e.preventDefault();
        e.stopPropagation();
    }

    function endSpeed(state: TouchState) {
        const p = player.value;
        if (p && !p.isDisposed()) p.playbackRate(state.prevRate);
    }

    function cancelTouch() {
        clearLongPress();
        if (touch?.mode === 'speed') endSpeed(touch);
        if (touch && touch.mode !== 'pending' && touch.mode !== 'ignore') hideSoon(0);
        touch = null;
    }

    function startLongPress() {
        longPressTimer = null;
        const p = player.value;
        if (!touch || touch.mode !== 'pending' || !p || p.paused()) return;
        touch.mode = 'speed';
        touch.prevRate = p.playbackRate() ?? 1;
        p.playbackRate(GESTURE_LONG_PRESS_RATE);
        show({ kind: 'speed', rate: GESTURE_LONG_PRESS_RATE });
    }

    function onTouchStart(e: TouchEvent) {
        lastTouchAt = performance.now();
        const p = player.value;
        if (!enabled.value || !p || !rootEl || !p.hasStarted()) return;
        if (e.touches.length > 1) {
            cancelTouch();
            return;
        }
        if (!isGestureTarget(e.target)) return;
        const t = e.changedTouches.item(0);
        if (!t) return;
        clearLongPress();
        touch = {
            id: t.identifier,
            startX: t.clientX,
            startY: t.clientY,
            startAt: performance.now(),
            rect: rootEl.getBoundingClientRect(),
            mode: 'pending',
            startTime: 0,
            startLevel: 0,
            targetTime: 0,
            prevRate: 1,
        };
        longPressTimer = setTimeout(startLongPress, GESTURE_LONG_PRESS_MS);
    }

    function lockMode(state: TouchState, axis: GestureAxis, p: Player): TouchMode {
        if (axis === 'x') {
            state.startTime = p.currentTime() ?? 0;
            state.targetTime = state.startTime;
            return 'scrub';
        }
        // Inline, a vertical drag scrolls the page
        if (!p.isFullscreen()) return 'ignore';
        if (gestureSide(state.startX - state.rect.left, state.rect.width) === 'right') {
            if (!canSetVolume(p)) return 'ignore';
            state.startLevel = p.muted() ? 0 : (p.volume() ?? 1);
            return 'volume';
        }
        state.startLevel = brightness.value;
        return 'brightness';
    }

    function onTouchMove(e: TouchEvent) {
        const state = touch;
        const p = player.value;
        if (!state || !p) return;
        if (e.touches.length > 1) {
            cancelTouch();
            return;
        }
        const t = findTouch(e.changedTouches, state.id);
        if (!t) return;
        const dx = t.clientX - state.startX;
        const dy = t.clientY - state.startY;

        if (state.mode === 'speed') {
            consume(e);
            return;
        }
        if (state.mode === 'pending') {
            // The page started scrolling under the finger: no gesture, no long-press
            if (!e.cancelable) {
                clearLongPress();
                state.mode = 'ignore';
                return;
            }
            const axis = gestureLockAxis(dx, dy);
            if (!axis) return;
            clearLongPress();
            state.mode = lockMode(state, axis, p);
        }
        if (state.mode === 'ignore') return;
        consume(e);

        if (state.mode === 'scrub') {
            const duration = p.duration() ?? 0;
            state.targetTime = gestureScrubTime(state.startTime, dx, state.rect.width, duration);
            show({
                kind: 'scrub',
                time: state.targetTime,
                delta: state.targetTime - state.startTime,
                duration,
                cue: findGestureCue(vttCues.value, state.targetTime),
            });
        } else if (state.mode === 'volume') {
            const level = gestureSwipeLevel(state.startLevel, dy, state.rect.height, 0, 1);
            p.volume(level);
            if (level > 0 && p.muted()) p.muted(false);
            show({ kind: 'volume', level });
        } else if (state.mode === 'brightness') {
            brightness.value = gestureSwipeLevel(
                state.startLevel,
                dy,
                state.rect.height,
                GESTURE_BRIGHTNESS_MIN,
                GESTURE_BRIGHTNESS_MAX,
            );
            show({ kind: 'brightness', level: brightness.value });
        }
    }

    function handleTap(e: TouchEvent, state: TouchState, t: Touch, p: Player) {
        const now = performance.now();
        if (
            !gestureIsTap(t.clientX - state.startX, t.clientY - state.startY, now - state.startAt)
        ) {
            return;
        }
        const step = normalizeGestureSeekStep(seekStep.value);
        const zone = gestureTapZone(state.startX - state.rect.left, state.rect.width);
        const next = gestureSeekForTap(
            zone,
            now,
            lastTap?.at ?? null,
            lastTap?.zone ?? null,
            streak,
            step,
        );
        lastTap = { at: now, zone };
        if (!next) {
            // A plain tap: video.js toggles the controls as usual
            streak = null;
            return;
        }
        consume(e);
        streak = next;
        const delta = next.side === 'right' ? step : -step;
        p.currentTime(gestureClampTime((p.currentTime() ?? 0) + delta, p.duration() ?? 0));
        seekId += 1;
        show({ kind: 'seek', side: next.side, amount: next.amount, id: seekId });
        hideSoon(GESTURE_SEEK_STREAK_MS);
    }

    function onTouchEnd(e: TouchEvent) {
        lastTouchAt = performance.now();
        const state = touch;
        const p = player.value;
        if (!state || !p) return;
        const t = findTouch(e.changedTouches, state.id);
        if (!t) return;
        clearLongPress();
        touch = null;

        switch (state.mode) {
            case 'speed':
                consume(e);
                endSpeed(state);
                hideSoon(0);
                break;
            case 'scrub':
                consume(e);
                p.currentTime(state.targetTime);
                hideSoon(350);
                break;
            case 'volume':
            case 'brightness':
                consume(e);
                hideSoon(500);
                break;
            case 'pending':
                handleTap(e, state, t, p);
                break;
            default:
                break;
        }
    }

    function onTouchCancel() {
        lastTouchAt = performance.now();
        cancelTouch();
    }

    // Long-press opens the context menu on Android: not while a gesture may be running
    function onContextMenu(e: Event) {
        if (!enabled.value || !isGestureTarget(e.target)) return;
        if (touch || performance.now() - lastTouchAt < 800) e.preventDefault();
    }

    function applyBrightness(level: number) {
        const video = techEl();
        if (!video) return;
        video.style.filter = Math.abs(level - 1) < 0.01 ? '' : `brightness(${level.toFixed(2)})`;
    }

    function setup() {
        const p = player.value;
        if (!p || rootEl) return;
        rootEl = p.el() as HTMLElement;
        rootEl.addEventListener('touchstart', onTouchStart, { capture: true, passive: true });
        rootEl.addEventListener('touchmove', onTouchMove, { capture: true, passive: false });
        rootEl.addEventListener('touchend', onTouchEnd, { capture: true, passive: false });
        rootEl.addEventListener('touchcancel', onTouchCancel, { capture: true });
        rootEl.addEventListener('contextmenu', onContextMenu, { capture: true });

        stopWatchers.push(
            watch(
                enabled,
                (on) => {
                    rootEl?.classList.toggle('vjs-gestures', on);
                    if (!on) {
                        cancelTouch();
                        brightness.value = 1;
                    }
                },
                { immediate: true },
            ),
            watch(brightness, applyBrightness),
        );
    }

    function cleanup() {
        cancelTouch();
        if (hideTimer) clearTimeout(hideTimer);
        hideTimer = null;
        indicator.value = null;
        stopWatchers.splice(0).forEach((stop) => stop());
        if (rootEl) {
            rootEl.removeEventListener('touchstart', onTouchStart, { capture: true });
            rootEl.removeEventListener('touchmove', onTouchMove, { capture: true });
            rootEl.removeEventListener('touchend', onTouchEnd, { capture: true });
            rootEl.removeEventListener('touchcancel', onTouchCancel, { capture: true });
            rootEl.removeEventListener('contextmenu', onContextMenu, { capture: true });
            rootEl.classList.remove('vjs-gestures');
        }
        rootEl = null;
        lastTap = null;
        streak = null;
    }

    return { indicator, brightness, setup, cleanup };
}
