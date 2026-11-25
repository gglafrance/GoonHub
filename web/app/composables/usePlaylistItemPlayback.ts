import type videojs from 'video.js';
import type { PlaylistQueueItem } from '~/utils/playlistQueue';

type Player = ReturnType<typeof videojs>;

interface PlaylistItemPlaybackOptions {
    /** The video.js player of the watch page */
    player: () => Player | null;
    /** The playlist item being played (null outside a playlist) */
    item: () => PlaylistQueueItem | null;
    /** The scene shown by the watch page */
    sceneId: () => number;
    /** A marker of a marker playlist reached the end of its range */
    onMarkerEnd: () => void;
    /** Saves the resume point (the item and the time in its scene) */
    save: (item: PlaylistQueueItem, positionS: number) => void;
    /** Throttle of the resume point saves while playing */
    saveIntervalMs?: number;
}

/**
 * Playback of the current playlist item on the watch page (docs/MARKER_PLAYLISTS.md):
 * - a marker plays from its start to its end, then onMarkerEnd fires once (playing past the
 *   end by a seek does not count, so the user can keep watching);
 * - a marker of the scene already shown is seeked to directly;
 * - the resume point is saved when an item starts, every saveIntervalMs while it plays, and
 *   when the page is left.
 */
export const usePlaylistItemPlayback = (options: PlaylistItemPlaybackOptions) => {
    const saveIntervalMs = options.saveIntervalMs ?? 10000;
    let attached: Player | null = null;
    let lastTime = 0;
    let lastSave = 0;
    // Item whose end was handled (reset when the item changes)
    let endedKey: string | null = null;

    const currentTime = () => attached?.currentTime() ?? 0;

    /** Ends the current marker (range end reached, or end of its scene); once per item. */
    const finishMarker = () => {
        const item = options.item();
        if (!item || item.end === null || endedKey === item.key) return;
        endedKey = item.key;
        options.onMarkerEnd();
    };

    const onTimeUpdate = () => {
        const item = options.item();
        const t = currentTime();
        if (item && item.sceneId === options.sceneId()) {
            if (item.end !== null && crossedRangeEnd(item.end, lastTime, t)) finishMarker();
            const now = Date.now();
            if (now - lastSave >= saveIntervalMs) {
                lastSave = now;
                options.save(item, t);
            }
        }
        lastTime = t;
    };

    watch(
        options.player,
        (player) => {
            if (attached === player) return;
            attached?.off('timeupdate', onTimeUpdate);
            attached = player;
            lastTime = currentTime();
            attached?.on('timeupdate', onTimeUpdate);
        },
        { immediate: true },
    );

    watch(
        () => options.item()?.key,
        (_key, previousKey) => {
            const item = options.item();
            endedKey = null;
            if (!item) return;
            // The first item keeps the resume point until it plays (it may start from it)
            if (previousKey !== undefined) {
                lastSave = Date.now();
                options.save(item, item.start ?? 0);
            }
            // Moved to another marker of the scene on screen: seek to it (a new scene, or the
            // first item, starts at the `t` of the watch page query, possibly a resume point)
            const player = attached;
            if (
                previousKey !== undefined &&
                item.start !== null &&
                player &&
                item.sceneId === options.sceneId()
            ) {
                if (Math.abs(currentTime() - item.start) > 0.5) player.currentTime(item.start);
                lastTime = item.start;
                player.play()?.catch(() => {});
            }
        },
        { immediate: true },
    );

    // A new scene starts from its own time
    watch(options.sceneId, () => {
        lastTime = 0;
    });

    onBeforeUnmount(() => {
        const item = options.item();
        if (item && item.sceneId === options.sceneId() && attached) {
            options.save(item, currentTime());
        }
        attached?.off('timeupdate', onTimeUpdate);
        attached = null;
    });

    return { finishMarker };
};
