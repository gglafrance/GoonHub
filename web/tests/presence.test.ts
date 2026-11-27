import { describe, expect, test } from 'bun:test';
import {
    activityLabel,
    applyPresenceView,
    awayActivityLabel,
    formatPresenceTime,
    groupPresence,
    isPresent,
    presenceGroupOf,
    presencePosition,
    presenceProgress,
    presenceWatchPath,
} from '../app/utils/presence';
import type { PresenceActivity, PresenceView } from '../app/types/presence';

const view = (id: number, over: Partial<PresenceView> = {}): PresenceView => ({
    user: { id, username: `user${id}` },
    visible: true,
    status: 'online',
    seq: 1,
    ...over,
});

const watching = (over: Partial<PresenceActivity> = {}): PresenceActivity => ({
    type: 'watching_scene',
    started_at: '2026-11-26T12:00:00Z',
    scene: { id: 7, title: 'Seven', duration: 600 },
    position: 100,
    position_at: '2026-11-26T12:00:00Z',
    paused: false,
    ...over,
});

const at = (iso: string) => Date.parse(iso);

describe('applyPresenceView', () => {
    test('keeps the most recent view', () => {
        const map: Record<number, PresenceView> = {};
        expect(applyPresenceView(map, view(1, { seq: 5 }))).toBe(true);
        expect(applyPresenceView(map, view(1, { seq: 3, status: 'away' }))).toBe(false);
        expect(map[1]!.status).toBe('online');
        expect(applyPresenceView(map, view(1, { seq: 6, status: 'away' }))).toBe(true);
        expect(map[1]!.status).toBe('away');
    });

    test('a hidden view is kept so that an older one cannot bring it back', () => {
        const map: Record<number, PresenceView> = {};
        applyPresenceView(map, view(1, { seq: 4, visible: false, status: undefined }));
        expect(applyPresenceView(map, view(1, { seq: 2 }))).toBe(false);
        expect(map[1]!.visible).toBe(false);
    });
});

describe('groups', () => {
    test('group of a view', () => {
        expect(presenceGroupOf(view(1))).toBe('online');
        expect(presenceGroupOf(view(1, { status: 'away' }))).toBe('away');
        expect(presenceGroupOf(view(1, { activity: watching() }))).toBe('watching');
        // a video left paused: away, not watching
        expect(
            presenceGroupOf(view(1, { status: 'away', activity: watching({ paused: true }) })),
        ).toBe('away');
        expect(presenceGroupOf(view(1, { status: 'offline' }))).toBe('offline');
        expect(isPresent(view(1, { status: 'away' }))).toBe(true);
        expect(isPresent(view(1, { status: 'offline' }))).toBe(false);
        expect(isPresent(view(1, { visible: false }))).toBe(false);
        expect(isPresent(undefined)).toBe(false);
    });

    test('groupPresence orders the groups and their members', () => {
        const groups = groupPresence([
            view(1, {
                user: { id: 1, username: 'zed' },
                status: 'offline',
                last_seen_at: '2026-11-25T10:00:00Z',
            }),
            view(2, {
                user: { id: 2, username: 'amy' },
                status: 'offline',
                last_seen_at: '2026-11-26T10:00:00Z',
            }),
            view(3, { user: { id: 3, username: 'Bob' } }),
            view(4, { user: { id: 4, username: 'al' } }),
            view(5, { status: 'away' }),
            view(6, { activity: watching() }),
            view(7, { visible: false }),
            view(8, { status: 'offline' }),
        ]);
        expect(groups.map((g) => g.key)).toEqual(['watching', 'online', 'away', 'offline']);
        expect(groups.map((g) => g.label)).toEqual(['Watching', 'Online', 'Away', 'Offline']);
        expect(groups[1]!.views.map((v) => v.user.username)).toEqual(['al', 'Bob']);
        expect(groups[3]!.views.map((v) => v.user.id)).toEqual([2, 1, 8]);
        expect(groups.flatMap((g) => g.views).some((v) => v.user.id === 7)).toBe(false);
        expect(groupPresence([])).toEqual([]);
    });
});

describe('playback position', () => {
    test('extrapolated while playing, capped at the duration', () => {
        const a = watching();
        expect(presencePosition(a, at('2026-11-26T12:00:30Z'))).toBe(130);
        expect(presencePosition(a, at('2026-11-26T13:00:00Z'))).toBe(600);
        expect(presenceProgress(a, at('2026-11-26T12:00:20Z'))).toBe(20);
    });

    test('frozen while paused, null when hidden', () => {
        expect(presencePosition(watching({ paused: true }), at('2026-11-26T12:10:00Z'))).toBe(100);
        const hidden: PresenceActivity = {
            type: 'watching_scene',
            started_at: '2026-11-26T12:00:00Z',
        };
        expect(presencePosition(hidden, Date.now())).toBeNull();
        expect(presenceProgress(hidden, Date.now())).toBeNull();
        expect(activityLabel(hidden)).toBe('Watching something');
        expect(presenceWatchPath(hidden, true, Date.now())).toBeNull();
    });

    test('labels and links', () => {
        expect(activityLabel(watching())).toBe('Seven');
        expect(awayActivityLabel(watching({ paused: true }))).toBe('Paused on Seven');
        expect(awayActivityLabel(watching())).toBe('On Seven');
        expect(
            awayActivityLabel({ type: 'watching_scene', started_at: '2026-11-26T12:00:00Z' }),
        ).toBe('Watching something');
        expect(activityLabel(watching({ scene: { id: 7, title: '', duration: 0 } }))).toBe(
            'Untitled scene',
        );
        const now = at('2026-11-26T12:00:05Z');
        expect(presenceWatchPath(watching(), false, now)).toBe('/watch/7');
        expect(presenceWatchPath(watching(), true, now)).toBe('/watch/7?t=105');
        expect(presenceWatchPath(watching({ position: 0, paused: true }), true, now)).toBe(
            '/watch/7',
        );
        expect(formatPresenceTime(65)).toBe('1:05');
        expect(formatPresenceTime(3725)).toBe('1:02:05');
        expect(formatPresenceTime(-3)).toBe('0:00');
    });
});
