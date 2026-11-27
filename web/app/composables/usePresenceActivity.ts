import type { ShallowRef } from 'vue';
import type videojs from 'video.js';
import type { Scene } from '~/types/scene';

type Player = ReturnType<typeof videojs>;

interface PresenceActivityOptions {
    player: ShallowRef<Player | null>;
    scene: Ref<Scene | undefined>;
    /** Reports the "now watching" activity only when true (the watch page; not a share link). */
    enabled: () => boolean;
}

/**
 * Presence of a player (docs/PRESENCE.md):
 * - every player tells the presence store whether it really plays (not paused, not ended, not
 *   stalled for `stalled_after`): a playing video keeps the user online without input, in any
 *   tab (watch page, Goon2gether lobby...);
 * - when enabled, it also reports what this tab is watching: once playback started, the scene,
 *   position and pause state on play, pause, seek, end and scene change, and every activity
 *   interval meanwhile (also while paused, to keep the activity alive). Cleared when the player
 *   is destroyed or the page unloads.
 */
export const usePresenceActivity = (options: PresenceActivityOptions) => {
    const { player, scene, enabled } = options;
    const authStore = useAuthStore();
    const presenceStore = usePresenceStore();
    const api = useApiPresence();

    // scene of the activity reported by this tab (null: none)
    let reportedSceneId: number | null = null;
    let lastReport = 0;
    let timer: ReturnType<typeof setInterval> | null = null;
    // the last time the position moved (a buffering player is not paused but does not play)
    let lastProgressAt = 0;
    let lastPosition = -1;
    let unregisterPlayback: (() => void) | null = null;

    const report = (force: boolean) => {
        if (!enabled() || !authStore.isAuthenticated) return;
        const p = player.value;
        const s = scene.value;
        if (!p || !s) return;
        const now = Date.now();
        if (!force && now - lastReport < presenceStore.activityIntervalMs) return;
        lastReport = now;
        reportedSceneId = s.id;
        api.reportActivity({
            tab_id: presenceStore.tabId,
            type: 'watching_scene',
            scene_id: s.id,
            position: Math.max(0, Math.round((p.currentTime() ?? 0) * 10) / 10),
            // an ended video does not play
            paused: p.paused() || p.ended(),
        }).catch(() => {
            // presence is not critical
        });
    };

    const clear = () => {
        if (reportedSceneId === null) return;
        reportedSceneId = null;
        api.clearActivity(presenceStore.tabId).catch(() => {});
    };

    const isPlaying = () => {
        const p = player.value;
        if (!p) return false;
        return isPlaybackActive({
            paused: p.paused(),
            ended: p.ended(),
            lastProgressAt,
            now: Date.now(),
            stalledAfterMs: presenceStore.stalledAfterMs,
        });
    };

    const onPlay = () => {
        lastProgressAt = Date.now();
        report(true);
    };
    const onPlaying = () => {
        lastProgressAt = Date.now();
    };
    const onChange = () => {
        if (reportedSceneId !== null) report(true);
    };
    const onTimeUpdate = () => {
        const position = player.value?.currentTime() ?? 0;
        if (position !== lastPosition) {
            lastPosition = position;
            lastProgressAt = Date.now();
        }
        if (reportedSceneId !== null) report(false);
    };

    // a new scene in the same player (playlist): reported at once when one was
    watch(
        () => scene.value?.id,
        (id, previous) => {
            if (id !== previous && reportedSceneId !== null) report(true);
        },
    );

    const setup = () => {
        const p = player.value;
        if (!p) return;
        p.on('play', onPlay);
        p.on('playing', onPlaying);
        p.on('pause', onChange);
        p.on('seeked', onChange);
        p.on('ended', onChange);
        p.on('timeupdate', onTimeUpdate);
        timer = setInterval(onChange, presenceStore.activityIntervalMs);
        window.addEventListener('pagehide', clear);
        unregisterPlayback?.();
        unregisterPlayback = presenceStore.registerPlayback(isPlaying);
    };

    const cleanup = () => {
        const p = player.value;
        if (p) {
            p.off('play', onPlay);
            p.off('playing', onPlaying);
            p.off('pause', onChange);
            p.off('seeked', onChange);
            p.off('ended', onChange);
            p.off('timeupdate', onTimeUpdate);
        }
        if (timer) clearInterval(timer);
        timer = null;
        window.removeEventListener('pagehide', clear);
        unregisterPlayback?.();
        unregisterPlayback = null;
        clear();
    };

    return { setup, cleanup };
};
