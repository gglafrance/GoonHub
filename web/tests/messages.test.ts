import { describe, expect, test } from 'bun:test';
import type { Conversation, Message, MessageAttachment } from '../app/types/messages';
import {
    applyNewMessage,
    buildThreadItems,
    conversationTitle,
    isEmoteOnly,
    linkifyText,
    markerShareSubject,
    mergeMessages,
    messageDayKey,
    messagePreviewText,
    parseClockInput,
    readOnlyText,
    relativeDay,
    shareCandidates,
    shareCardInfo,
    shareCardLink,
    sortConversations,
    tokenizeMessage,
    unavailableShareText,
    unreadBadge,
} from '../app/utils/messages';

const ME = 1;
const BOB = 2;

function msg(id: number, sender: number | null, at: string, extra: Partial<Message> = {}): Message {
    return {
        id,
        conversation_id: 7,
        sender_id: sender,
        body: `m${id}`,
        attachment: null,
        edited_at: null,
        deleted: false,
        created_at: at,
        ...extra,
    };
}

function conv(id: number, extra: Partial<Conversation> = {}): Conversation {
    return {
        id,
        kind: 'direct',
        other: { id: BOB, username: 'bob', display_name: '', avatar_version: 0 },
        last_message: null,
        unread_count: 0,
        muted: false,
        last_read_message_id: null,
        other_last_read_message_id: null,
        can_send: true,
        updated_at: '2026-11-26T10:00:00Z',
        ...extra,
    };
}

describe('mergeMessages', () => {
    test('dedupes by id, replaces, sorts', () => {
        const a = [msg(3, ME, '2026-11-26T10:00:00Z'), msg(1, ME, '2026-11-26T09:00:00Z')];
        const out = mergeMessages(a, [
            msg(2, BOB, '2026-11-26T09:30:00Z'),
            msg(3, ME, '2026-11-26T10:00:00Z', { body: 'edited' }),
        ]);
        expect(out.map((m) => m.id)).toEqual([1, 2, 3]);
        expect(out[2]!.body).toBe('edited');
    });
});

describe('buildThreadItems', () => {
    // local times, so that the day separators do not depend on the time zone of the test run
    const d = (day: number, h: number, min = 0) => new Date(2026, 10, day, h, min).toISOString();

    test('day separators and groups by sender within 5 minutes', () => {
        const items = buildThreadItems(
            [
                msg(1, BOB, d(25, 22, 0)),
                msg(2, BOB, d(25, 22, 3)),
                msg(3, BOB, d(25, 22, 30)), // more than 5 minutes later: new group
                msg(4, ME, d(25, 22, 31)),
                msg(5, ME, d(26, 9, 0)), // next day: separator, new group
            ],
            ME,
            null,
        );
        expect(items.map((i) => i.kind)).toEqual([
            'day',
            'message',
            'message',
            'message',
            'message',
            'day',
            'message',
        ]);
        const messages = items.filter((i) => i.kind === 'message');
        expect(messages.map((i) => (i.kind === 'message' ? i.groupStart : null))).toEqual([
            true,
            false,
            true,
            true,
            true,
        ]);
        expect(messages.map((i) => (i.kind === 'message' ? i.groupEnd : null))).toEqual([
            false,
            true,
            true,
            true,
            true,
        ]);
        expect(messages.map((i) => (i.kind === 'message' ? i.mine : null))).toEqual([
            false,
            false,
            false,
            true,
            true,
        ]);
    });

    test('"seen" under my last message once the other read it', () => {
        const list = [msg(1, ME, d(26, 9)), msg(2, ME, d(26, 9, 1)), msg(3, BOB, d(26, 9, 2))];
        const seen = (read: number | null) =>
            buildThreadItems(list, ME, read)
                .filter((i) => i.kind === 'message' && i.seen)
                .map((i) => (i.kind === 'message' ? i.message.id : 0));
        expect(seen(null)).toEqual([]);
        expect(seen(1)).toEqual([]);
        expect(seen(2)).toEqual([2]);
        expect(seen(3)).toEqual([2]);
    });

    test('empty thread', () => {
        expect(buildThreadItems([], ME, 4)).toEqual([]);
    });
});

describe('relativeDay', () => {
    test('today, yesterday, older', () => {
        const now = new Date(2026, 10, 26, 12);
        expect(relativeDay(messageDayKey(new Date(2026, 10, 26, 1)), now)).toBe('today');
        expect(relativeDay(messageDayKey(new Date(2026, 10, 25, 23)), now)).toBe('yesterday');
        expect(relativeDay(messageDayKey(new Date(2026, 10, 20)), now)).toBeNull();
    });
});

describe('message text', () => {
    const emotes = new Map([['pog', { name: 'Pog', url: '/emotes/pog.png' }]]);

    test('links: http(s) only, trailing punctuation left out', () => {
        expect(linkifyText('see https://example.com/a?b=1, ok')).toEqual([
            { kind: 'text', text: 'see ' },
            { kind: 'link', text: 'https://example.com/a?b=1', href: 'https://example.com/a?b=1' },
            { kind: 'text', text: ', ok' },
        ]);
        expect(linkifyText('javascript:alert(1) ftp://x.y')).toEqual([
            { kind: 'text', text: 'javascript:alert(1) ftp://x.y' },
        ]);
        expect(linkifyText('(http://a.b/c).')[1]).toEqual({
            kind: 'link',
            text: 'http://a.b/c',
            href: 'http://a.b/c',
        });
    });

    test('HTML stays text', () => {
        const tokens = tokenizeMessage('<img src=x onerror=alert(1)> :pog:', emotes);
        expect(tokens[0]).toEqual({ kind: 'text', text: '<img src=x onerror=alert(1)> ' });
        expect(tokens[1]).toEqual({ kind: 'emote', name: 'Pog', url: '/emotes/pog.png' });
    });

    test('emotes only', () => {
        expect(isEmoteOnly(tokenizeMessage(':pog: :POG:', emotes))).toBe(true);
        expect(isEmoteOnly(tokenizeMessage(':pog: hi', emotes))).toBe(false);
        expect(isEmoteOnly(tokenizeMessage(':unknown:', emotes))).toBe(false);
    });
});

describe('share cards', () => {
    test('links open the right place', () => {
        const scene: MessageAttachment = {
            kind: 'scene',
            available: true,
            scene: { id: 4, title: 'T', duration: 600 },
        };
        expect(shareCardLink(scene)).toBe('/watch/4');
        expect(shareCardLink({ ...scene, time: 75.6 })).toBe('/watch/4?t=75');
        const marker: MessageAttachment = {
            kind: 'marker',
            available: true,
            marker: {
                id: 9,
                scene_id: 4,
                scene_title: 'T',
                scene_duration: 600,
                start: 10,
                end: 40,
                label: 'Kiss',
                color: '',
                owner_username: 'bob',
            },
        };
        expect(shareCardLink(marker)).toBe('/watch/4?t=10&end=40');
        expect(shareCardLink({ ...marker, marker: { ...marker.marker!, end: null } })).toBe(
            '/watch/4?t=10',
        );
        expect(
            shareCardLink({
                kind: 'playlist',
                available: true,
                playlist: {
                    uuid: 'u-1',
                    name: 'P',
                    kind: 'scenes',
                    visibility: 'public',
                    owner_username: 'bob',
                    item_count: 2,
                },
            }),
        ).toBe('/playlists/u-1');
        expect(shareCardLink({ kind: 'lobby', available: true })).toBeNull();
        expect(shareCardLink({ kind: 'scene', available: false, reason: 'trashed' })).toBeNull();
    });

    test('card texts', () => {
        const info = shareCardInfo({
            kind: 'marker',
            available: true,
            marker: {
                id: 9,
                scene_id: 4,
                scene_title: 'Scene',
                scene_duration: 600,
                start: 70,
                end: 95,
                label: 'Kiss',
                color: '',
                owner_username: 'bob',
                thumbnail_url: '/marker-thumbnails/9?v=1',
            },
        });
        expect(info).toMatchObject({
            title: 'Kiss',
            subtitle: 'Scene',
            badge: '1:10 - 1:35',
            thumbnail: '/marker-thumbnails/9?v=1',
        });
        expect(
            shareCardInfo({
                kind: 'scene',
                available: true,
                scene: { id: 1, title: 'T', duration: 3700 },
                time: 65,
            }),
        ).toMatchObject({ subtitle: 'From 1:05', badge: '1:01:40' });
        // a lobby without the playing title (no scenes:view) shows no thumbnail
        expect(
            shareCardInfo({
                kind: 'lobby',
                available: true,
                lobby: {
                    id: 'l',
                    name: 'Friday',
                    visibility: 'private',
                    online_count: 2,
                    now_playing: { scene_id: 3 },
                    is_member: false,
                    joinable: true,
                },
            }).thumbnail,
        ).toBeNull();
    });

    test('not available texts', () => {
        expect(unavailableShareText({ kind: 'scene', available: false, reason: 'trashed' })).toBe(
            'This video is in the trash',
        );
        expect(
            unavailableShareText({ kind: 'playlist', available: false, reason: 'private' }),
        ).toBe('This playlist is private');
        expect(
            unavailableShareText({ kind: 'marker', available: false, reason: 'no_permission' }),
        ).toBe('This marker is not available to you');
        expect(unavailableShareText({ kind: 'lobby', available: false, reason: 'not_found' })).toBe(
            'This lobby is closed',
        );
        expect(
            unavailableShareText({ kind: 'lobby', available: false, reason: 'not_joinable' }),
        ).toBe('This invite no longer works');
    });

    test('marker subject', () => {
        expect(
            markerShareSubject({
                id: 9,
                scene_id: 4,
                label: 'Kiss',
                timestamp: 10,
                end_timestamp: null,
                scene_title: 'S',
            }),
        ).toEqual({
            kind: 'marker',
            markerId: 9,
            sceneId: 4,
            label: 'Kiss',
            start: 10,
            end: null,
            sceneTitle: 'S',
        });
    });
});

describe('conversations', () => {
    test('previews', () => {
        const p = {
            id: 1,
            sender_id: ME,
            body: '',
            attachment_kind: null,
            deleted: false,
            created_at: '',
        } as const;
        expect(messagePreviewText(null, ME)).toBe('No message yet');
        expect(messagePreviewText({ ...p, body: 'hi\n there' }, ME)).toBe('You: hi there');
        expect(messagePreviewText({ ...p, sender_id: BOB, attachment_kind: 'marker' }, ME)).toBe(
            'Shared a marker',
        );
        expect(messagePreviewText({ ...p, deleted: true }, ME)).toBe('You: Message deleted');
    });

    test('a new message updates the conversation', () => {
        const c = conv(7, {
            unread_count: 1,
            last_message: {
                id: 5,
                sender_id: BOB,
                body: 'old',
                attachment_kind: null,
                deleted: false,
                created_at: '',
            },
        });
        const fromBob = applyNewMessage(c, msg(6, BOB, '2026-11-26T11:00:00Z'), ME, false);
        expect(fromBob.unread_count).toBe(2);
        expect(fromBob.last_message?.id).toBe(6);
        expect(fromBob.updated_at).toBe('2026-11-26T11:00:00Z');
        expect(applyNewMessage(c, msg(6, BOB, '2026-11-26T11:00:00Z'), ME, true).unread_count).toBe(
            1,
        );
        const mine = applyNewMessage(c, msg(6, ME, '2026-11-26T11:00:00Z'), ME, false);
        expect(mine.unread_count).toBe(1);
        expect(mine.last_read_message_id).toBe(6);
        // an older event does not move it back
        expect(applyNewMessage(fromBob, msg(4, BOB, '2026-11-26T09:00:00Z'), ME, false)).toBe(
            fromBob,
        );
    });

    test('sorting, titles, read-only texts, badge', () => {
        const sorted = sortConversations([
            conv(1, { updated_at: '2026-11-25T10:00:00Z' }),
            conv(2, { updated_at: '2026-11-26T10:00:00Z' }),
        ]);
        expect(sorted.map((c) => c.id)).toEqual([2, 1]);
        expect(conversationTitle({ other: null })).toBe('Deleted user');
        expect(
            conversationTitle(
                conv(1, {
                    other: { id: 2, username: 'bob', display_name: 'Bobby', avatar_version: 0 },
                }),
            ),
        ).toBe('Bobby');
        expect(readOnlyText('policy', 'bob')).toContain('bob does not accept messages');
        expect(readOnlyText(undefined, 'bob')).toBe('');
        expect(unreadBadge(0)).toBe('');
        expect(unreadBadge(120)).toBe('99+');
    });

    test('share candidates: recent first, then friends, deduplicated and filtered', () => {
        const recent = [
            conv(1),
            conv(3, {
                other: { id: 5, username: 'eve', display_name: 'Eve', avatar_version: 0 },
                can_send: false,
            }),
            conv(4, { other: null }),
        ];
        const friends = [
            {
                id: BOB,
                username: 'bob',
                display_name: '',
                avatar_version: 0,
                can_message: true,
                recent: false,
            },
            {
                id: 6,
                username: 'carol',
                display_name: '',
                avatar_version: 0,
                can_message: true,
                recent: false,
            },
        ];
        const all = shareCandidates(recent, friends, '');
        expect(all.map((c) => [c.id, c.recent, c.can_message])).toEqual([
            [BOB, true, true],
            [5, true, false],
            [6, false, true],
        ]);
        expect(shareCandidates(recent, friends, 'CAR').map((c) => c.id)).toEqual([6]);
        expect(shareCandidates(recent, friends, 'eve').map((c) => c.id)).toEqual([5]);
    });
});

describe('parseClockInput', () => {
    test('seconds, m:ss, h:mm:ss', () => {
        expect(parseClockInput('75')).toBe(75);
        expect(parseClockInput('1:15')).toBe(75);
        expect(parseClockInput(' 1:01:40 ')).toBe(3700);
        expect(parseClockInput('0:00')).toBe(0);
    });
    test('invalid', () => {
        for (const v of ['', 'abc', '1:75', '1::2', '1:2:3:4', '-5', '1.5']) {
            expect(parseClockInput(v)).toBeNull();
        }
    });
});
