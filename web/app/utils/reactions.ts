// Pure helpers of the reactions on chat messages: the messages between users
// (docs/MESSAGES.md, "Reactions") and the Goon2gether chat (docs/GOON2GETHER.md, protocol v3).
// Not the floating reactions over the Goon2gether video. Tests in web/tests/reactions.test.ts.
import type { MessageReaction, ReactionUser } from '~/types/messages';
import type { ChatReaction } from '~/types/together';

/** A reaction as shown under a message: the key, the count, mine or not, who reacted. */
export interface ReactionPill {
    /** an emoji, or `:name:` of a server emote */
    key: string;
    count: number;
    /** I reacted with this key (clicking the pill removes my reaction) */
    mine: boolean;
    /** who reacted, in reaction order ("You" for me) */
    names: string[];
}

/** Caps of the server: distinct keys per message, reactions of one user per message. */
export interface ReactionLimits {
    maxKinds: number;
    maxPerUser: number;
}

/** Pills of a message between users (my id: "You"). */
export function messageReactionPills(
    reactions: readonly MessageReaction[] | null | undefined,
    myId: number,
): ReactionPill[] {
    return (reactions ?? [])
        .filter((r) => r.count > 0)
        .map((r) => ({
            key: r.emoji,
            count: r.count,
            mine: r.users.some((u) => u.id === myId),
            names: r.users.map((u) => (u.id === myId ? 'You' : u.display_name || u.username)),
        }));
}

/**
 * Pills of a Goon2gether chat message. `isMine` recognizes me (by number when I am anonymous:
 * the payloads carry no id for an anonymous member). A moderator gets the real name of an
 * anonymous reactor, shown with the alias.
 */
export function lobbyReactionPills(
    reactions: readonly ChatReaction[] | null | undefined,
    isMine: (id: number | null | undefined, anon?: number | null) => boolean,
): ReactionPill[] {
    return (reactions ?? [])
        .filter((r) => r.count > 0)
        .map((r) => {
            let mine = false;
            const names = r.users.map((u) => {
                if (isMine(u.id, u.anon_number)) {
                    mine = true;
                    return 'You';
                }
                if (u.anonymous && u.id != null && u.anon_number != null) {
                    return `Anonymous #${u.anon_number} (${u.username})`;
                }
                return u.username;
            });
            return { key: r.emote, count: r.count, mine, names };
        });
}

/** Names for a tooltip: "You", "You and bob", "alice, bob and carol", "a, b, c and 4 others". */
export function reactorNames(names: readonly string[], count: number, shown = 3): string {
    const list = names.slice(0, shown);
    const others = Math.max(0, count - list.length);
    if (others > 0) {
        return `${list.join(', ')} and ${others} other${others === 1 ? '' : 's'}`;
    }
    if (list.length <= 1) return list[0] ?? '';
    return `${list.slice(0, -1).join(', ')} and ${list[list.length - 1]}`;
}

/** Tooltip of a pill: who reacted and with what. */
export function reactionTitle(pill: ReactionPill): string {
    return `${reactorNames(pill.names, pill.count)} reacted with ${pill.key}`;
}

/** What clicking a key does: remove my reaction when I reacted with it, else add it. */
export function reactionAction(pills: readonly ReactionPill[], key: string): 'add' | 'remove' {
    return pills.some((p) => p.key === key && p.mine) ? 'remove' : 'add';
}

/** How many reactions I have on a message. */
export function myReactionCount(pills: readonly ReactionPill[]): number {
    return pills.filter((p) => p.mine).length;
}

/**
 * Whether I may add a reaction with this key (the server checks the same caps): a new key
 * needs room for one more kind, and I need room for one more reaction. Without a key: whether
 * a new key can be picked (the "Add reaction" button; on a full message, the pills remain).
 */
export function canAddReaction(
    pills: readonly ReactionPill[],
    limits: ReactionLimits,
    key?: string,
): boolean {
    if (key !== undefined && pills.some((p) => p.key === key && p.mine)) return false;
    if (limits.maxPerUser > 0 && myReactionCount(pills) >= limits.maxPerUser) return false;
    const exists = key !== undefined && pills.some((p) => p.key === key);
    return exists || limits.maxKinds <= 0 || pills.length < limits.maxKinds;
}

/**
 * Applies my reaction (add or remove) to the reactions of a message between users: the
 * optimistic update, replaced by the server's answer. Adding appends me to the key (or a new
 * key at the end), removing drops me and the key once empty.
 */
export function toggleMessageReaction(
    reactions: readonly MessageReaction[] | null | undefined,
    key: string,
    me: ReactionUser,
    add: boolean,
): MessageReaction[] {
    const list = (reactions ?? []).map((r) => ({ ...r, users: [...r.users] }));
    const i = list.findIndex((r) => r.emoji === key);
    const current = i >= 0 ? list[i] : undefined;
    const has = !!current?.users.some((u) => u.id === me.id);
    if (add) {
        if (has) return list;
        if (current) {
            current.users.push(me);
            current.count = current.users.length;
        } else {
            list.push({ emoji: key, count: 1, users: [me] });
        }
        return list;
    }
    if (!current || !has) return list;
    current.users = current.users.filter((u) => u.id !== me.id);
    current.count = current.users.length;
    return current.count === 0 ? list.filter((_, j) => j !== i) : list;
}
