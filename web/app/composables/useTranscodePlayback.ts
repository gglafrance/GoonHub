import videojs from 'video.js';
import type { PlaybackNotice, ScenePlayback, ScenePlaybackMode } from '~/types/scene';

type Player = ReturnType<typeof videojs>;

// Source type of a stream converted on the fly. The middleware below turns it into a plain
// video/mp4 source and hides the start offset of the stream from the rest of the player.
const TRANSCODE_MIME = 'video/x-goonhub-transcode';

// Seeks are debounced (scrubbing, repeated arrow keys) since each one restarts ffmpeg.
const SEEK_DEBOUNCE_MS = 350;
// A conversion that ends this many seconds before the end of the scene broke off
const PREMATURE_END_MARGIN_S = 5;

interface TranscodeSource {
    src: string;
    type: string;
    goonhubStart?: number;
}

interface TimeRangesLike {
    length: number;
    start: (i: number) => number;
    end: (i: number) => number;
}

interface TechLike {
    currentTime: () => number;
    setCurrentTime: (time: number) => void;
}

// Per player state read by the middleware.
interface TranscodeController {
    baseUrl: string;
    // position in the scene of the start of the current stream
    offset: number;
    // scene duration in seconds (0: unknown)
    duration: number;
    // seek waiting for its reload (shown as the current time meanwhile)
    pendingSeek: number | null;
    tech: TechLike | null;
    // the next seek of the tech is dropped (the stream is replaced instead)
    skipTechSeek: boolean;
    requestSeek: (time: number) => void;
}

// Video.js typings miss the getter form of hasStarted() and error(null) (clears the error)
interface PlayerStateApi {
    hasStarted: (request?: boolean) => boolean;
    error: (err?: null) => unknown;
}
const playerState = (p: Player) => p as unknown as PlayerStateApi;

const controllers = new WeakMap<Player, TranscodeController>();
let middlewareRegistered = false;

const createTimeRanges = (ranges: number[][]) =>
    (videojs.time.createTimeRanges as unknown as (r: number[][]) => TimeRangesLike)(ranges);

const transcodeUrl = (baseUrl: string, start: number) => {
    if (start <= 0) return baseUrl;
    return `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}start=${start.toFixed(3)}`;
};

// The stream of a conversion starts at the requested position (0 for the browser), so the
// middleware adds the offset to the time and duration the player sees, and turns seeks into
// a reload of the stream at the new position.
const registerMiddleware = () => {
    if (middlewareRegistered) return;
    middlewareRegistered = true;

    videojs.use(TRANSCODE_MIME, (player: Player) => {
        const ctl = () => controllers.get(player);
        return {
            setSource(src: TranscodeSource, next: (err: unknown, src?: unknown) => void) {
                const c = ctl();
                if (!c) {
                    next(new Error('No transcode controller'));
                    return;
                }
                c.offset = Math.max(0, Number(src.goonhubStart) || 0);
                c.pendingSeek = null;
                c.skipTechSeek = false;
                next(null, { src: transcodeUrl(c.baseUrl, c.offset), type: 'video/mp4' });
            },
            setTech(tech: TechLike) {
                const c = ctl();
                if (!c || c.tech === tech) return;
                c.tech = tech;
                // Middleware setters can't cancel a seek: drop it at the tech. Any seek on the
                // stream (even inside the buffered data) makes the browser request it again
                // from its first byte, since it can't be fetched by range.
                const setCurrentTime = tech.setCurrentTime.bind(tech);
                tech.setCurrentTime = (time: number) => {
                    const current = ctl();
                    if (current?.skipTechSeek) {
                        current.skipTechSeek = false;
                        return;
                    }
                    setCurrentTime(time);
                };
            },
            currentTime(time: number) {
                const c = ctl();
                if (!c) return time;
                return c.pendingSeek ?? time + c.offset;
            },
            duration(duration: number) {
                const c = ctl();
                if (!c) return duration;
                if (c.duration > 0) return c.duration;
                return Number.isFinite(duration) ? duration + c.offset : duration;
            },
            buffered(ranges: TimeRangesLike) {
                const c = ctl();
                if (!c || !ranges) return ranges;
                const shifted: number[][] = [];
                for (let i = 0; i < ranges.length; i++) {
                    shifted.push([ranges.start(i) + c.offset, ranges.end(i) + c.offset]);
                }
                return createTimeRanges(shifted);
            },
            seekable(ranges: TimeRangesLike) {
                const c = ctl();
                if (!c || !(c.duration > 0)) return ranges;
                return createTimeRanges([[0, c.duration]]);
            },
            setCurrentTime(time: number) {
                const c = ctl();
                if (!c) return time;
                // Seeks reload the stream at the new position
                c.requestSeek(time);
                c.skipTechSeek = true;
                return time - c.offset;
            },
        };
    });
};

interface UseTranscodePlaybackOptions {
    playback: () => ScenePlayback | null | undefined;
    transcodeUrl: () => string | undefined;
    duration: () => number;
    loop: () => boolean;
}

// Plays the scenes browsers can't play through the on-the-fly conversion endpoint, and
// falls back to it when the browser fails on a file considered playable.
export const useTranscodePlayback = (
    player: Ref<Player | null>,
    options: UseTranscodePlaybackOptions,
) => {
    // Random id of this player: the server replaces the conversion of the same player on seek
    const sessionId = Math.random().toString(36).slice(2, 12);

    // Mode of the current source (null: nothing playable loaded)
    const mode = ref<ScenePlaybackMode | null>(null);
    const notice = ref<PlaybackNotice | null>(null);
    const active = computed(() => mode.value === 'remux' || mode.value === 'transcode');

    let controller: TranscodeController | null = null;
    let forceTranscode = false;
    let seekTimer: ReturnType<typeof setTimeout> | null = null;
    let lastTime = 0;
    let lastAutoRetry = 0;

    const needsConversion = (playback: ScenePlayback | null | undefined) =>
        !!playback && playback.mode !== 'direct' && !!options.transcodeUrl();

    // Whether the original can be loaded as is (computed before the player exists, so that the
    // <source> of a file that needs a conversion is never downloaded)
    const startsDirect = !needsConversion(options.playback());

    const baseUrl = () => {
        const url = options.transcodeUrl() ?? '';
        const params = `sid=${sessionId}${forceTranscode ? '&mode=transcode' : ''}`;
        return `${url}${url.includes('?') ? '&' : '?'}${params}`;
    };

    const clearSeekTimer = () => {
        if (seekTimer) {
            clearTimeout(seekTimer);
            seekTimer = null;
        }
    };

    // Video.js normally autoplays on every loadstart: not wanted when reloading a paused
    // conversion. Also keeps the "started" state so the poster does not come back.
    const keepStateOnReload = (p: Player, wasPaused: boolean) => {
        const started = playerState(p).hasStarted();
        const opts = (p as unknown as { options_: { autoplay: unknown } }).options_;
        p.one('loadstart', () => {
            if (started) p.hasStarted(true);
            if (wasPaused && opts.autoplay) {
                const saved = opts.autoplay;
                opts.autoplay = false;
                setTimeout(() => {
                    opts.autoplay = saved;
                }, 0);
            }
        });
    };

    // Asks the server (HEAD) whether a conversion slot is free and where the stream really
    // starts (a remux can only start on a keyframe, up to a few seconds before).
    const preflight = async (start: number) => {
        const url = controller ? transcodeUrl(controller.baseUrl, start) : '';
        try {
            const res = await fetch(url, { method: 'HEAD', credentials: 'include' });
            const realStart = Number.parseFloat(res.headers.get('X-Transcode-Start') ?? '');
            return { status: res.status, start: Number.isFinite(realStart) ? realStart : start };
        } catch {
            return { status: 0, start };
        }
    };

    let loadToken = 0;

    const loadTranscode = async (start: number, play: boolean) => {
        const p = player.value;
        if (!p || !controller) return;
        clearSeekTimer();
        notice.value = null;
        const token = ++loadToken;
        controller.baseUrl = baseUrl();
        controller.duration = options.duration();
        lastTime = start;
        mode.value = forceTranscode ? 'transcode' : (options.playback()?.mode ?? 'transcode');
        if (active.value) controller.pendingSeek = start;

        const check = await preflight(start);
        // A newer load (seek, scene change) took over meanwhile
        if (token !== loadToken || player.value !== p || !controller) return;
        if (check.status === 503) {
            controller.pendingSeek = null;
            p.pause();
            notice.value = { kind: 'limit' };
            return;
        }
        if (check.status === 403) {
            p.pause();
            showDisabled(options.playback()?.reason);
            return;
        }

        const rate = p.playbackRate() ?? 1;
        p.loop(false);
        keepStateOnReload(p, !play);
        p.one('loadedmetadata', () => {
            if (rate !== 1) p.playbackRate(rate);
        });
        lastTime = check.start;
        const source: TranscodeSource = {
            src: controller.baseUrl,
            type: TRANSCODE_MIME,
            goonhubStart: check.start,
        };
        p.src(source);
        if (play) p.play()?.catch(() => {});
    };

    const requestSeek = (time: number) => {
        if (!controller) return;
        controller.pendingSeek = time;
        clearSeekTimer();
        seekTimer = setTimeout(() => {
            seekTimer = null;
            const p = player.value;
            if (!p) return;
            loadTranscode(time, !p.paused());
        }, SEEK_DEBOUNCE_MS);
    };

    const showDisabled = (reason?: string) => {
        notice.value = { kind: 'disabled', reason };
        mode.value = null;
    };

    // Call once the player exists.
    const attach = () => {
        const p = player.value;
        if (!p) return;
        registerMiddleware();
        controller = {
            baseUrl: baseUrl(),
            offset: 0,
            duration: options.duration(),
            pendingSeek: null,
            tech: null,
            skipTechSeek: false,
            requestSeek,
        };
        controllers.set(p, controller);
        p.on('timeupdate', () => {
            lastTime = p.currentTime() ?? lastTime;
        });
        // The element can't loop a stream that starts in the middle of the scene
        p.on('ended', () => {
            if (!active.value || !controller) return;
            const reached = p.currentTime() ?? lastTime;
            const duration = options.duration();
            if (duration > 0 && reached < duration - PREMATURE_END_MARGIN_S) {
                // The conversion stopped well before the end of the scene: ffmpeg failed
                // on this file. Never loop on it (that restarted it forever).
                conversionFailed(reached);
                return;
            }
            if (options.loop()) loadTranscode(0, true);
        });
    };

    // Loads the scene: the original, or the conversion when the browser can't play it.
    // Returns false when nothing can be played (notice shown).
    const load = (directUrl: string, start: number, play: boolean) => {
        const p = player.value;
        if (!p) return false;
        clearSeekTimer();
        loadToken++;
        forceTranscode = false;
        notice.value = null;
        const playback = options.playback();
        if (needsConversion(playback)) {
            if (!playback!.transcode_enabled) {
                p.pause();
                showDisabled(playback!.reason);
                return false;
            }
            loadTranscode(start, play);
            return true;
        }
        mode.value = 'direct';
        p.loop(options.loop());
        p.src({ type: 'video/mp4', src: directUrl });
        return true;
    };

    // Initial state when the original is not loaded by the <source> element.
    const init = (start: number, play: boolean) => {
        if (startsDirect) {
            mode.value = 'direct';
            return;
        }
        load('', start, play);
    };

    // A conversion that broke off: a remux (streams copied) is tried once more as a full
    // transcode from where it stopped; when that fails too, the failure is shown.
    const conversionFailed = (at: number) => {
        const p = player.value;
        if (!p) return;
        p.pause();
        if (!forceTranscode && mode.value === 'remux') {
            forceTranscode = true;
            loadTranscode(at, true);
            return;
        }
        notice.value = { kind: 'failed' };
    };

    const diagnose = async (start: number, play: boolean) => {
        const url = controller ? transcodeUrl(controller.baseUrl, start) : '';
        let status = 0;
        try {
            const res = await fetch(url, { method: 'HEAD', credentials: 'include' });
            status = res.status;
        } catch {
            status = 0;
        }
        if (status === 503) {
            notice.value = { kind: 'limit' };
        } else if (status === 403) {
            showDisabled(options.playback()?.reason);
        } else if (status === 200 && !forceTranscode && mode.value === 'remux') {
            // The copied streams can't be packaged: re-encode them instead
            conversionFailed(start);
        } else if (status === 200 && Date.now() - lastAutoRetry > 15000) {
            // Transient failure (e.g. the browser lost the stream): start again where it was
            lastAutoRetry = Date.now();
            loadTranscode(start, play);
        } else {
            notice.value = { kind: 'failed' };
        }
    };

    // Handles a player error. Returns true when handled (no generic error to show).
    const handleError = () => {
        const p = player.value;
        if (!p) return false;
        const code = p.error()?.code ?? 0;
        // 3: decode error, 4: source not supported
        if (mode.value === 'direct') {
            if ((code !== 3 && code !== 4) || !options.transcodeUrl()) return false;
            const playback = options.playback();
            playerState(p).error(null);
            if (playback && !playback.transcode_enabled) {
                showDisabled('Your browser could not play this file');
                return true;
            }
            // The stored codecs looked fine but the browser can't decode the file
            forceTranscode = true;
            loadTranscode(lastTime, true);
            return true;
        }
        if (active.value) {
            const start = controller?.pendingSeek ?? lastTime;
            const play = !p.paused() || playerState(p).hasStarted();
            playerState(p).error(null);
            p.pause();
            diagnose(start, play);
            return true;
        }
        return false;
    };

    const retry = () => {
        const p = player.value;
        if (!p || !controller) return;
        loadTranscode(lastTime, true);
    };

    const detach = () => {
        clearSeekTimer();
        if (player.value) controllers.delete(player.value);
        controller = null;
    };

    return {
        mode,
        active,
        notice,
        startsDirect,
        attach,
        init,
        load,
        handleError,
        retry,
        detach,
    };
};
