import { describe, expect, test } from 'bun:test';
import {
    avatarInitials,
    avatarUrl,
    bioSegments,
    blocksSharingPrivacy,
    clampOffset,
    cropFromView,
    displayNameOf,
    formatWatchTime,
    moveItem,
} from '../app/utils/profile';
import type { ProfileLayoutBlock } from '../app/types/profile';

describe('avatarUrl', () => {
    test('null without an avatar, versioned otherwise', () => {
        expect(avatarUrl(3, 0)).toBeNull();
        expect(avatarUrl(undefined, 5)).toBeNull();
        expect(avatarUrl(3, 1700000000000, 'lg')).toBe(
            '/api/v1/profiles/avatar/3/lg?v=1700000000000',
        );
    });
});

describe('names', () => {
    test('initials of up to two words', () => {
        expect(avatarInitials('nade')).toBe('N');
        expect(avatarInitials('Nade the pup')).toBe('NT');
        expect(avatarInitials('dark_wolf')).toBe('DW');
        expect(avatarInitials('  ')).toBe('?');
    });
    test('display name falls back to the username', () => {
        expect(displayNameOf({ username: 'bob', display_name: '  ' })).toBe('bob');
        expect(displayNameOf({ username: 'bob', display_name: 'Bob' })).toBe('Bob');
    });
});

describe('bioSegments', () => {
    test('http(s) and www links, trailing punctuation kept as text', () => {
        expect(bioSegments('see https://ex.com/a, or www.ex.org.')).toEqual([
            { text: 'see ' },
            { text: 'https://ex.com/a', href: 'https://ex.com/a' },
            { text: ', or ' },
            { text: 'www.ex.org', href: 'https://www.ex.org/' },
            { text: '.' },
        ]);
    });
    test('no link for other schemes, no HTML', () => {
        const segs = bioSegments('javascript:alert(1) <b>x</b> data:text/html,hi');
        expect(segs.every((s) => !s.href)).toBe(true);
        expect(segs.map((s) => s.text).join('')).toBe(
            'javascript:alert(1) <b>x</b> data:text/html,hi',
        );
    });
    test('plain text stays one segment', () => {
        expect(bioSegments('hello\nworld')).toEqual([{ text: 'hello\nworld' }]);
    });
});

describe('formatWatchTime', () => {
    test('minutes, hours, days', () => {
        expect(formatWatchTime(59)).toBe('0m');
        expect(formatWatchTime(45 * 60)).toBe('45m');
        expect(formatWatchTime(3 * 3600 + 20 * 60)).toBe('3h 20m');
        expect(formatWatchTime(2 * 86400 + 4 * 3600)).toBe('2d 4h');
        expect(formatWatchTime(86400)).toBe('1d');
    });
});

describe('moveItem', () => {
    test('moves and leaves the input alone', () => {
        const list = ['a', 'b', 'c', 'd'];
        expect(moveItem(list, 0, 2)).toEqual(['b', 'c', 'a', 'd']);
        expect(moveItem(list, 3, 0)).toEqual(['d', 'a', 'b', 'c']);
        expect(moveItem(list, 1, 9)).toEqual(list);
        expect(list).toEqual(['a', 'b', 'c', 'd']);
    });
});

describe('blocksSharingPrivacy', () => {
    test('other blocks of the same privacy item', () => {
        const layout = ['recent_watches', 'most_watched', 'stats'].map(
            (type): ProfileLayoutBlock => ({
                type,
                visible: true,
                locked_placeholder: false,
                config: {},
            }),
        );
        const privacy: Record<string, string> = {
            recent_watches: 'activity',
            most_watched: 'activity',
            stats: 'stats',
        };
        const of = (t: string) => privacy[t];
        expect(blocksSharingPrivacy(layout, of, 'recent_watches')).toEqual(['most_watched']);
        expect(blocksSharingPrivacy(layout, of, 'stats')).toEqual([]);
    });
});

describe('crop', () => {
    test('crop rectangle of the view', () => {
        // 2000x1000 image in a 320x320 frame: cover scale 0.32, zoom 2 -> 0.64
        const crop = cropFromView(
            { width: 2000, height: 1000 },
            { width: 320, height: 320 },
            0.64,
            -320,
            -64,
        );
        expect(crop).toEqual({ x: 500, y: 100, w: 500, h: 500 });
    });
    test('crop stays inside the image', () => {
        const crop = cropFromView(
            { width: 100, height: 100 },
            { width: 320, height: 320 },
            3.2,
            50,
            -1000,
        );
        expect(crop).toEqual({ x: 0, y: 0, w: 100, h: 100 });
    });
    test('offsets keep the frame covered', () => {
        expect(clampOffset(10, 500, 320)).toBe(0);
        expect(clampOffset(-400, 500, 320)).toBe(-180);
        expect(clampOffset(-50, 500, 320)).toBe(-50);
    });
});
