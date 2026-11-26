// Pure helpers of the presence (docs/PRESENCE.md). Tests in web/tests/presence.test.ts.
import type { PresenceActivity, PresenceGroupKey, PresenceView } from '~/types/presence';

/** Presence SSE events, targeted at the signed-in user (also listed in sse-worker.js). */
export const PRESENCE_SSE_EVENTS = ['presence:update'] as const;

/**
 * Applies a presence view to the map of views by user id, keeping the most recent one (highest
 * seq). Hidden views are kept too (visible false), so that an older event cannot bring a
 * revoked presence back. Returns whether the view was applied.
 */
export function applyPresenceView(map: Record<number, PresenceView>, view: PresenceView): boolean {
    const current = map[view.user.id];
    if (current && current.seq > view.seq) return false;
    map[view.user.id] = view;
    return true;
}

export interface PresenceGroup {
    key: PresenceGroupKey;
    label: string;
    views: PresenceView[];
}

const GROUP_LABELS: Record<PresenceGroupKey, string> = {
    watching: 'Watching',
    online: 'Online',
    idle: 'Idle',
    offline: 'Offline',
};

/** Group of a visible view: watching (online with an activity), online, idle or offline. */
export function presenceGroupOf(view: PresenceView): PresenceGroupKey {
    if (!view.visible || !view.status || view.status === 'offline') return 'offline';
    if (view.activity) return 'watching';
    return view.status === 'idle' ? 'idle' : 'online';
}

/** Whether the user is online or idle (not offline, nor hidden). */
export function isPresent(view: PresenceView | undefined): boolean {
    return !!view && view.visible && !!view.status && view.status !== 'offline';
}

/**
 * Groups the visible views: Watching, Online, Idle (by username), then Offline (most recently
 * seen first). Empty groups are left out.
 */
export function groupPresence(views: PresenceView[]): PresenceGroup[] {
    const byKey: Record<PresenceGroupKey, PresenceView[]> = {
        watching: [],
        online: [],
        idle: [],
        offline: [],
    };
    for (const v of views) {
        if (v.visible) byKey[presenceGroupOf(v)].push(v);
    }
    const byName = (a: PresenceView, b: PresenceView) =>
        a.user.username.localeCompare(b.user.username, undefined, { sensitivity: 'base' });
    byKey.watching.sort(byName);
    byKey.online.sort(byName);
    byKey.idle.sort(byName);
    byKey.offline.sort((a, b) => {
        const ta = a.last_seen_at ? Date.parse(a.last_seen_at) : 0;
        const tb = b.last_seen_at ? Date.parse(b.last_seen_at) : 0;
        return tb - ta || byName(a, b);
    });
    return (Object.keys(byKey) as PresenceGroupKey[])
        .filter((k) => byKey[k].length > 0)
        .map((k) => ({ key: k, label: GROUP_LABELS[k], views: byKey[k] }));
}

/**
 * Current playback position (seconds) of a watching activity: the reported position, plus the
 * time elapsed since it was reported while playing (capped at the scene duration). null when
 * the position is hidden.
 */
export function presencePosition(activity: PresenceActivity, nowMs: number): number | null {
    if (activity.position === undefined) return null;
    let pos = activity.position;
    if (!activity.paused && activity.position_at) {
        pos += Math.max(0, (nowMs - Date.parse(activity.position_at)) / 1000);
    }
    const duration = activity.scene?.duration ?? 0;
    if (duration > 0) pos = Math.min(pos, duration);
    return Math.max(0, pos);
}

/** Progress of a watching activity in percent (0-100), null when unknown. */
export function presenceProgress(activity: PresenceActivity, nowMs: number): number | null {
    const pos = presencePosition(activity, nowMs);
    const duration = activity.scene?.duration ?? 0;
    if (pos === null || duration <= 0) return null;
    return Math.min(100, (pos / duration) * 100);
}

/** m:ss or h:mm:ss. */
export function formatPresenceTime(seconds: number): string {
    const s = Math.max(0, Math.floor(seconds));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = String(s % 60).padStart(2, '0');
    return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
}

/** What a user is doing, in words. */
export function activityLabel(activity: PresenceActivity): string {
    if (activity.type === 'watching_scene') {
        return activity.scene ? activity.scene.title || 'Untitled scene' : 'Watching something';
    }
    return 'Busy';
}

/**
 * Watch page link of a watching activity ("Watch this too"): from the start, or at the current
 * position of the user. null when the scene is hidden.
 */
export function presenceWatchPath(
    activity: PresenceActivity,
    atPosition: boolean,
    nowMs: number,
): string | null {
    if (activity.type !== 'watching_scene' || !activity.scene) return null;
    const base = `/watch/${activity.scene.id}`;
    if (!atPosition) return base;
    const pos = presencePosition(activity, nowMs);
    return pos && pos >= 1 ? `${base}?t=${Math.floor(pos)}` : base;
}

/** A random id for this tab (the activity of each tab is kept apart). */
export function newPresenceTabId(): string {
    return (Math.random().toString(36).slice(2) + Date.now().toString(36)).slice(0, 32);
}
