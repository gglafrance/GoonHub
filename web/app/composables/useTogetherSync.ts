import type videojs from 'video.js';
import type { TogetherRoom } from '~/composables/useTogetherRoom';

type Player = ReturnType<typeof videojs>;

// Player methods wrapped on the instance (own properties shadowing the prototype ones)
interface ControlledApi {
    play: () => Promise<void> | undefined;
    pause: () => unknown;
    currentTime: (time?: number) => number;
    playbackRate: (rate?: number) => number;
}
const CONTROLLED = ['play', 'pause', 'currentTime', 'playbackRate'] as const;

const TICK_MS = 500;
// A player call this soon after a user input (click, key, touch) comes from the user
const USER_INPUT_WINDOW_MS = 1000;
// After a local action of a controller, the server state is not enforced until its answer
// arrives (or this long)
const LOCAL_ACTION_HOLD_MS = 1500;
// Seeks while dragging the progress bar are sent at most this often (trailing)
const SEEK_SEND_DEBOUNCE_MS = 200;
// Paused: the player is moved to the lobby position when off by more than this
const PAUSED_TOLERANCE_S = 0.3;
// Fallback when a new item never reports loadedmetadata
const ITEM_LOAD_TIMEOUT_MS = 15000;

interface SyncOptions {
    player: ComputedRef<Player | null>;
    // the scene is converted on the fly: seeks restart the conversion
    transcoding: ComputedRef<boolean>;
    room: TogetherRoom;
    // queue item the player was mounted for
    itemId: Ref<number | null>;
}

/**
 * Controlled playback of a Goon2gether room (docs/GOON2GETHER.md "Sync algorithm"): applies
 * the server state to the video.js player, corrects the drift (seek or tiny rate nudges),
 * turns the user's play, pause, seek and rate actions into commands when the user has control
 * and blocks them otherwise, and reports buffering, readiness and the end of the item.
 */
export const useTogetherSync = (options: SyncOptions) => {
    const { player, transcoding, room, itemId } = options;
    const { socket, canControl, currentItem } = room;

    const autoplayBlocked = ref(false);

    let programmatic = 0;
    let lastInputAt = 0;
    let itemLoading = true;
    let itemLoadTimer: ReturnType<typeof setTimeout> | null = null;
    let localActionUntil = 0;
    let seekCooldownUntil = 0;
    let nudging = false;
    let lastStatusKey = '';
    let seekSendTimer: ReturnType<typeof setTimeout> | null = null;
    let pendingSeek: number | null = null;
    let tickTimer: ReturnType<typeof setInterval> | null = null;
    let detach: (() => void) | null = null;

    const prog = <T>(fn: () => T): T => {
        programmatic++;
        try {
            return fn();
        } finally {
            programmatic--;
        }
    };

    const userInitiated = () =>
        programmatic === 0 && !itemLoading && Date.now() - lastInputAt < USER_INPUT_WINDOW_MS;

    const notAllowed = () =>
        socket.showError({
            code: 'not_allowed',
            message: 'Only the host and members with control can change playback',
        });

    const markLocalAction = () => {
        localActionUntil = Date.now() + LOCAL_ACTION_HOLD_MS;
    };

    const flushSeek = () => {
        seekSendTimer = null;
        if (pendingSeek === null) return;
        socket.send({ t: 'seek', position: Math.max(0, pendingSeek) });
        pendingSeek = null;
    };

    // Each returns whether the player should carry the call out
    const onUserPlay = (p: Player): boolean => {
        const s = socket.state.value;
        if (!s || s.item_id == null || s.playing) return true;
        if (!canControl.value) {
            notAllowed();
            return false;
        }
        markLocalAction();
        socket.send({ t: 'play', position: rawTime(p) });
        return true;
    };

    const onUserPause = (p: Player): boolean => {
        const s = socket.state.value;
        if (!s || s.item_id == null || !s.playing) return true;
        if (!canControl.value) {
            notAllowed();
            return false;
        }
        markLocalAction();
        socket.send({ t: 'pause', position: rawTime(p) });
        return true;
    };

    const onUserSeek = (time: number): boolean => {
        const s = socket.state.value;
        if (!s || s.item_id == null) return true;
        if (!canControl.value) {
            notAllowed();
            return false;
        }
        markLocalAction();
        seekCooldownUntil = Date.now() + LOCAL_ACTION_HOLD_MS;
        pendingSeek = time;
        if (!seekSendTimer) seekSendTimer = setTimeout(flushSeek, SEEK_SEND_DEBOUNCE_MS);
        return true;
    };

    const onUserRate = (rate: number): boolean => {
        const s = socket.state.value;
        if (!s || s.item_id == null || Math.abs(s.rate - rate) < 0.001) return true;
        if (!canControl.value) {
            notAllowed();
            return false;
        }
        markLocalAction();
        socket.send({ t: 'rate', rate });
        return true;
    };

    // Original (prototype) methods, bypassing the wrappers
    const proto = (p: Player) => Object.getPrototypeOf(p) as ControlledApi;
    const rawTime = (p: Player) => proto(p).currentTime.call(p) ?? 0;

    const wrap = (p: Player) => {
        const api = p as unknown as ControlledApi;
        const base = proto(p);
        api.play = function () {
            if (userInitiated() && !onUserPlay(p)) return Promise.resolve();
            return base.play.call(p);
        };
        api.pause = function () {
            if (userInitiated() && !onUserPause(p)) return p;
            return base.pause.call(p);
        };
        api.currentTime = function (time?: number) {
            if (time !== undefined && userInitiated() && !onUserSeek(time)) {
                return base.currentTime.call(p);
            }
            return time === undefined ? base.currentTime.call(p) : base.currentTime.call(p, time);
        };
        api.playbackRate = function (rate?: number) {
            if (rate !== undefined && userInitiated() && !onUserRate(rate)) {
                return base.playbackRate.call(p);
            }
            return rate === undefined ? base.playbackRate.call(p) : base.playbackRate.call(p, rate);
        };
        return () => {
            for (const name of CONTROLLED) Reflect.deleteProperty(api, name);
        };
    };

    const playProgrammatic = (p: Player) => {
        const res = prog(() => p.play());
        res?.catch(() => {
            if (p.isDisposed()) return;
            // Autoplay with sound blocked: play muted and offer to unmute
            prog(() => p.muted(true));
            autoplayBlocked.value = true;
            prog(() => p.play())?.catch(() => {});
        });
    };

    const unmute = () => {
        const p = player.value;
        if (p && !p.isDisposed()) p.muted(false);
        autoplayBlocked.value = false;
    };

    const setRate = (p: Player, rate: number) => {
        if (Math.abs((proto(p).playbackRate.call(p) ?? 1) - rate) > 0.001) {
            prog(() => p.playbackRate(rate));
        }
    };

    const seekTo = (p: Player, time: number) => {
        prog(() => p.currentTime(time));
        seekCooldownUntil = Date.now() + (transcoding.value ? 3000 : 1000);
    };

    const reportStatus = (p: Player) => {
        const s = socket.state.value;
        if (!s || s.item_id == null || s.item_id !== itemId.value) return;
        const ready = !itemLoading && p.readyState() >= 3 && !p.seeking();
        const key = `${s.item_id}:${s.seq}:${ready}`;
        if (key === lastStatusKey) return;
        if (socket.send({ t: 'status', buffering: !ready, seq: s.seq, item_id: s.item_id })) {
            lastStatusKey = key;
        }
    };

    const tick = () => {
        const p = player.value;
        const s = socket.state.value;
        const item = currentItem.value;
        if (!p || p.isDisposed() || !s || s.item_id == null || !item) return;
        if (item.id !== itemId.value) return;
        reportStatus(p);
        if (itemLoading || Date.now() < localActionUntil || p.readyState() < 1) return;

        const end = itemEnd(item);
        const expected = expectedPosition(s, socket.serverNow(), end);
        const shouldPlay = stateAdvances(s) && (end == null || expected < end - 0.05);
        if (shouldPlay && p.paused() && !p.ended()) playProgrammatic(p);
        else if (!shouldPlay && !p.paused()) prog(() => p.pause());

        if (p.seeking() || Date.now() < seekCooldownUntil) return;
        const current = rawTime(p);
        if (!shouldPlay) {
            nudging = false;
            setRate(p, s.rate);
            if (Math.abs(current - expected) > PAUSED_TOLERANCE_S && !p.ended()) {
                seekTo(p, expected);
            }
            return;
        }
        const action = driftCorrection({
            current,
            expected,
            rate: s.rate,
            threshold: socket.config.value.drift_threshold,
            nudge: socket.config.value.drift_nudge,
            transcoding: transcoding.value,
            nudging,
        });
        if (action.kind === 'seek') {
            nudging = false;
            setRate(p, s.rate);
            seekTo(p, action.position);
        } else if (action.kind === 'rate') {
            nudging = Math.abs(action.rate - s.rate) > 1e-6;
            setRate(p, action.rate);
        }
    };

    const finishItemLoad = () => {
        if (!itemLoading) return;
        if (itemLoadTimer) clearTimeout(itemLoadTimer);
        // Let the player settle (start time seek, conversion start) before syncing
        itemLoadTimer = setTimeout(() => {
            itemLoading = false;
            itemLoadTimer = null;
            tick();
        }, 300);
    };

    const attach = (p: Player) => {
        itemLoading = true;
        lastStatusKey = '';
        nudging = false;
        const unwrap = wrap(p);
        const onLoaded = () => finishItemLoad();
        const onChange = () => tick();
        p.on('loadedmetadata', onLoaded);
        p.on('canplay', onLoaded);
        for (const ev of ['waiting', 'canplay', 'playing', 'seeked', 'stalled']) p.on(ev, onChange);
        if (p.readyState() >= 1) finishItemLoad();
        itemLoadTimer = setTimeout(() => {
            itemLoading = false;
        }, ITEM_LOAD_TIMEOUT_MS);
        return () => {
            unwrap();
            if (p.isDisposed()) return;
            p.off('loadedmetadata', onLoaded);
            p.off('canplay', onLoaded);
            for (const ev of ['waiting', 'canplay', 'playing', 'seeked', 'stalled']) {
                p.off(ev, onChange);
            }
        };
    };

    watch(
        player,
        (p) => {
            detach?.();
            detach = null;
            if (itemLoadTimer) clearTimeout(itemLoadTimer);
            if (p) detach = attach(p);
        },
        { immediate: true },
    );

    // A new state is the reference again: local actions stop holding the corrections
    watch(
        () => socket.state.value?.seq,
        () => {
            localActionUntil = 0;
            seekCooldownUntil = Math.min(seekCooldownUntil, Date.now());
            tick();
        },
    );
    // A new connection has no status from us yet
    watch(socket.status, (status) => {
        if (status === 'open') {
            lastStatusKey = '';
            tick();
        }
    });

    const onInput = () => {
        lastInputAt = Date.now();
    };
    const inputEvents = ['pointerdown', 'keydown', 'touchstart', 'wheel'] as const;

    onMounted(() => {
        for (const ev of inputEvents) {
            window.addEventListener(ev, onInput, { capture: true, passive: true });
        }
        tickTimer = setInterval(tick, TICK_MS);
    });

    onBeforeUnmount(() => {
        for (const ev of inputEvents) window.removeEventListener(ev, onInput, { capture: true });
        if (tickTimer) clearInterval(tickTimer);
        if (itemLoadTimer) clearTimeout(itemLoadTimer);
        if (seekSendTimer) clearTimeout(seekSendTimer);
        detach?.();
        detach = null;
    });

    /** The player reached the end of the item (ScenePlayer `ended`). */
    const reportEnded = () => {
        if (itemId.value != null) socket.send({ t: 'ended', item_id: itemId.value });
    };

    return { autoplayBlocked, unmute, reportEnded };
};
