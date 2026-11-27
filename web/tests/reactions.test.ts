import { describe, expect, test } from 'bun:test';
import type { MessageReaction, ReactionUser } from '../app/types/messages';
import type { ChatReaction } from '../app/types/together';
import {
    canAddReaction,
    lobbyReactionPills,
    messageReactionPills,
    reactionAction,
    reactionTitle,
    reactorNames,
    toggleMessageReaction,
    type ReactionPill,
} from '../app/utils/reactions';

const ME: ReactionUser = { id: 1, username: 'me', display_name: '' };
const BOB: ReactionUser = { id: 2, username: 'bob', display_name: 'Bobby' };

const fire = (...users: ReactionUser[]): MessageReaction => ({
    emoji: '🔥',
    count: users.length,
    users,
});

describe('messageReactionPills', () => {
    test('marks mine and names the users', () => {
        const pills = messageReactionPills(
            [fire(BOB, ME), { emoji: ':pog:', count: 1, users: [BOB] }],
            ME.id,
        );
        expect(pills).toEqual([
            { key: '🔥', count: 2, mine: true, names: ['Bobby', 'You'] },
            { key: ':pog:', count: 1, mine: false, names: ['Bobby'] },
        ]);
    });

    test('missing or empty lists give no pill', () => {
        expect(messageReactionPills(undefined, 1)).toEqual([]);
        expect(messageReactionPills([{ emoji: '🔥', count: 0, users: [] }], 1)).toEqual([]);
    });
});

describe('lobbyReactionPills', () => {
    const you = { user_id: 5, anon_number: 3 };
    const isMine = (id: number | null | undefined, anon?: number | null) =>
        (id != null && id === you.user_id) || (anon != null && anon === you.anon_number);

    test('recognizes me by id or by anonymous number, redacted names stay aliases', () => {
        const reactions: ChatReaction[] = [
            {
                emote: '🔥',
                count: 2,
                users: [
                    { id: null, username: 'Anonymous #3', anonymous: true, anon_number: 3 },
                    { id: null, username: 'Anonymous #4', anonymous: true, anon_number: 4 },
                ],
            },
            { emote: ':pog:', count: 1, users: [{ id: 7, username: 'alice' }] },
        ];
        expect(lobbyReactionPills(reactions, isMine)).toEqual([
            { key: '🔥', count: 2, mine: true, names: ['You', 'Anonymous #4'] },
            { key: ':pog:', count: 1, mine: false, names: ['alice'] },
        ]);
    });

    test('a moderator sees the real name behind the alias', () => {
        const pills = lobbyReactionPills(
            [
                {
                    emote: '😂',
                    count: 1,
                    users: [{ id: 9, username: 'bob', anonymous: true, anon_number: 2 }],
                },
            ],
            () => false,
        );
        expect(pills[0]!.names).toEqual(['Anonymous #2 (bob)']);
    });
});

describe('tooltips', () => {
    test('reactorNames', () => {
        expect(reactorNames(['You'], 1)).toBe('You');
        expect(reactorNames(['You', 'bob'], 2)).toBe('You and bob');
        expect(reactorNames(['a', 'b', 'c'], 3)).toBe('a, b and c');
        expect(reactorNames(['a', 'b', 'c', 'd'], 4)).toBe('a, b, c and 1 other');
        expect(reactorNames(['a', 'b', 'c', 'd', 'e'], 7)).toBe('a, b, c and 4 others');
    });

    test('reactionTitle', () => {
        const pill: ReactionPill = { key: ':pog:', count: 2, mine: true, names: ['You', 'bob'] };
        expect(reactionTitle(pill)).toBe('You and bob reacted with :pog:');
    });
});

describe('toggling', () => {
    const pills: ReactionPill[] = [
        { key: '🔥', count: 2, mine: true, names: ['You', 'bob'] },
        { key: '😂', count: 1, mine: false, names: ['bob'] },
    ];

    test('reactionAction removes mine and adds the others', () => {
        expect(reactionAction(pills, '🔥')).toBe('remove');
        expect(reactionAction(pills, '😂')).toBe('add');
        expect(reactionAction(pills, '👍')).toBe('add');
    });

    test('canAddReaction follows the caps', () => {
        expect(canAddReaction(pills, { maxKinds: 20, maxPerUser: 10 }, '😂')).toBe(true);
        expect(canAddReaction(pills, { maxKinds: 20, maxPerUser: 10 }, '🔥')).toBe(false);
        // two kinds already: a new key is refused, an existing one is fine
        expect(canAddReaction(pills, { maxKinds: 2, maxPerUser: 10 }, '👍')).toBe(false);
        expect(canAddReaction(pills, { maxKinds: 2, maxPerUser: 10 }, '😂')).toBe(true);
        expect(canAddReaction(pills, { maxKinds: 2, maxPerUser: 10 })).toBe(false);
        // one reaction per user: I have mine
        expect(canAddReaction(pills, { maxKinds: 20, maxPerUser: 1 }, '😂')).toBe(false);
        expect(canAddReaction([], { maxKinds: 0, maxPerUser: 0 }, '👍')).toBe(true);
    });

    test('toggleMessageReaction adds and removes me', () => {
        const start = [fire(BOB)];
        const added = toggleMessageReaction(start, '🔥', ME, true);
        expect(added).toEqual([fire(BOB, ME)]);
        expect(start).toEqual([fire(BOB)]); // not mutated
        expect(toggleMessageReaction(added, '🔥', ME, true)).toEqual(added);

        const withNew = toggleMessageReaction(added, ':pog:', ME, true);
        expect(withNew.map((r) => r.emoji)).toEqual(['🔥', ':pog:']);

        expect(toggleMessageReaction(withNew, ':pog:', ME, false)).toEqual([fire(BOB, ME)]);
        expect(toggleMessageReaction(withNew, '🔥', ME, false)[0]).toEqual(fire(BOB));
        // removing what is not there changes nothing
        expect(toggleMessageReaction(start, '🔥', ME, false)).toEqual(start);
        expect(toggleMessageReaction(undefined, '🔥', ME, false)).toEqual([]);
        expect(toggleMessageReaction(undefined, '🔥', ME, true)).toEqual([fire(ME)]);
    });
});
