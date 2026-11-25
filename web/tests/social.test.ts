import { describe, expect, test } from 'bun:test';
import {
    badgeCount,
    friendAction,
    groupPrivacyItems,
    profilePath,
    relationshipBadges,
} from '../app/utils/social';
import type { PrivacyItem, Relationship } from '../app/types/social';

const rel = (over: Partial<Relationship> = {}): Relationship => ({
    self: false,
    following: false,
    followed_by: false,
    mutual: false,
    friend: false,
    request_sent: false,
    request_received: false,
    can_send_friend_request: true,
    ...over,
});

const keys = (r: Relationship) => relationshipBadges(r).map((b) => b.key);

describe('relationshipBadges', () => {
    test('nothing for a stranger or oneself', () => {
        expect(keys(rel())).toEqual([]);
        expect(keys(rel({ self: true, friend: true }))).toEqual([]);
    });

    test('follows you only when not mutual', () => {
        expect(keys(rel({ followed_by: true }))).toEqual(['follows_you']);
        expect(keys(rel({ followed_by: true, following: true, mutual: true }))).toEqual(['mutual']);
    });

    test('friend hides mutual', () => {
        expect(
            keys(rel({ friend: true, mutual: true, following: true, followed_by: true })),
        ).toEqual(['friend']);
    });

    test('pending requests', () => {
        expect(keys(rel({ request_received: true }))).toEqual(['request_received']);
        expect(keys(rel({ request_sent: true }))).toEqual(['request_sent']);
    });
});

describe('friendAction', () => {
    test('one action per state', () => {
        expect(friendAction(rel({ self: true }))).toBe('none');
        expect(friendAction(rel({ friend: true }))).toBe('remove');
        expect(friendAction(rel({ request_received: true }))).toBe('respond');
        expect(friendAction(rel({ request_sent: true }))).toBe('cancel');
        expect(friendAction(rel())).toBe('add');
        expect(friendAction(rel({ can_send_friend_request: false }))).toBe('none');
    });
});

describe('groupPrivacyItems', () => {
    test('keeps the catalog order', () => {
        const item = (key: string, group: string): PrivacyItem => ({
            key,
            group,
            label: key,
            description: '',
            default: 'public',
            audience: 'public',
            is_default: true,
        });
        const groups = groupPrivacyItems([
            item('profile', 'Profile'),
            item('friends_list', 'Connections'),
            item('stats', 'Profile'),
        ]);
        expect(groups.map((g) => g.group)).toEqual(['Profile', 'Connections']);
        expect(groups[0]!.items.map((i) => i.key)).toEqual(['profile', 'stats']);
    });
});

describe('badgeCount and profilePath', () => {
    test('badge count', () => {
        expect(badgeCount(0)).toBe('');
        expect(badgeCount(7)).toBe('7');
        expect(badgeCount(120)).toBe('99+');
    });

    test('profile path is encoded', () => {
        expect(profilePath('a b/c')).toBe('/u/a%20b%2Fc');
    });
});
