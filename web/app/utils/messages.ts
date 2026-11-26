// Pure helpers of the messages between users (docs/MESSAGES.md). Tests in
// web/tests/messages.test.ts.
import type {
    Conversation,
    Message,
    MessageAttachment,
    MessagePreview,
    ReadOnlyReason,
    ShareTarget,
} from '~/types/messages';
import type { ShareSubject } from '~/types/share';
import { formatClock, tokenizeChat } from './together';

/** Message SSE events, all targeted at the participants (also listed in sse-worker.js). */
export const MESSAGE_SSE_EVENTS = [
    'chat:message',
    'chat:message_updated',
    'chat:read',
    'chat:typing',
    'chat:conversation_updated',
] as const;

export type MessageSseEvent = (typeof MESSAGE_SSE_EVENTS)[number];

/** A typing indicator is shown this long after the last typing event. */
export const TYPING_TTL_MS = 6000;

/** Consecutive messages of one sender within this delay are grouped (one header). */
export const MESSAGE_GROUP_GAP_MS = 5 * 60 * 1000;

/** Inserts or replaces messages by id, sorted by id (oldest first). */
export function mergeMessages(list: readonly Message[], incoming: readonly Message[]): Message[] {
    const byId = new Map<number, Message>();
    for (const m of list) byId.set(m.id, m);
    for (const m of incoming) byId.set(m.id, m);
    return [...byId.values()].sort((a, b) => a.id - b.id);
}

/** Local calendar day of a date (YYYY-MM-DD), for the day separators. */
export function messageDayKey(iso: string | Date): string {
    const d = typeof iso === 'string' ? new Date(iso) : iso;
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${d.getFullYear()}-${m}-${day}`;
}

/** 'today', 'yesterday' or null (an older day: shown as a date). */
export function relativeDay(dayKey: string, now: Date = new Date()): 'today' | 'yesterday' | null {
    if (dayKey === messageDayKey(now)) return 'today';
    const y = new Date(now);
    y.setDate(y.getDate() - 1);
    return dayKey === messageDayKey(y) ? 'yesterday' : null;
}

export type ThreadItem =
    | { kind: 'day'; key: string; day: string; date: string }
    | {
          kind: 'message';
          key: string;
          message: Message;
          mine: boolean;
          // first of a group: shows the header (name, time)
          groupStart: boolean;
          // last of a group
          groupEnd: boolean;
          // my last message, read by the other participant
          seen: boolean;
      };

/**
 * The rows of a thread: a day separator before the first message of each day, messages
 * grouped by sender (consecutive, same day, within MESSAGE_GROUP_GAP_MS), and "seen" under my
 * last message once the other participant read it.
 */
export function buildThreadItems(
    messages: readonly Message[],
    myId: number,
    otherLastReadId: number | null,
): ThreadItem[] {
    const items: ThreadItem[] = [];
    let lastMineId = 0;
    for (const m of messages) if (m.sender_id === myId && m.id > lastMineId) lastMineId = m.id;
    const seenId = otherLastReadId != null && otherLastReadId >= lastMineId ? lastMineId : 0;

    let prev: Message | null = null;
    let prevDay = '';
    for (const m of messages) {
        const day = messageDayKey(m.created_at);
        if (day !== prevDay) {
            const before = items[items.length - 1];
            if (before && before.kind === 'message') before.groupEnd = true;
            items.push({ kind: 'day', key: `day-${day}`, day, date: m.created_at });
            prevDay = day;
            prev = null;
        }
        const sameGroup =
            prev !== null &&
            prev.sender_id === m.sender_id &&
            new Date(m.created_at).getTime() - new Date(prev.created_at).getTime() <=
                MESSAGE_GROUP_GAP_MS;
        if (!sameGroup) {
            const last = items[items.length - 1];
            if (last && last.kind === 'message') last.groupEnd = true;
        }
        items.push({
            kind: 'message',
            key: `m-${m.id}`,
            message: m,
            mine: m.sender_id === myId,
            groupStart: !sameGroup,
            groupEnd: false,
            seen: seenId !== 0 && m.id === seenId,
        });
        prev = m;
    }
    const last = items[items.length - 1];
    if (last && last.kind === 'message') last.groupEnd = true;
    return items;
}

export type MessageToken =
    | { kind: 'text'; text: string }
    | { kind: 'emote'; name: string; url: string }
    | { kind: 'link'; text: string; href: string };

const URL_PATTERN = /\bhttps?:\/\/[^\s<>"']+/gi;
// punctuation that ends a sentence rather than a URL
const URL_TRAILING = /[.,!?;:)\]}'"]+$/;

/** Splits a text into text and link tokens (http and https only, trailing punctuation left out). */
export function linkifyText(text: string): MessageToken[] {
    const out: MessageToken[] = [];
    let last = 0;
    for (const match of text.matchAll(URL_PATTERN)) {
        const index = match.index ?? 0;
        let url = match[0];
        const trail = URL_TRAILING.exec(url);
        if (trail) url = url.slice(0, url.length - trail[0].length);
        if (!url || !/^https?:\/\/[^/\s]+/i.test(url)) continue;
        if (index > last) out.push({ kind: 'text', text: text.slice(last, index) });
        out.push({ kind: 'link', text: url, href: url });
        last = index + url.length;
    }
    if (last < text.length) out.push({ kind: 'text', text: text.slice(last) });
    return out;
}

/**
 * Tokens of a message body: server emotes (`:name:`), links and text. Rendered as text nodes,
 * images and anchors: never as HTML.
 */
export function tokenizeMessage(
    text: string,
    emotes: ReadonlyMap<string, { name: string; url: string }>,
): MessageToken[] {
    const out: MessageToken[] = [];
    for (const t of tokenizeChat(text, emotes)) {
        if (t.kind === 'emote') out.push(t);
        else out.push(...linkifyText(t.text));
    }
    return out;
}

/** Only emotes (and spaces): shown bigger. */
export function isEmoteOnly(tokens: readonly MessageToken[]): boolean {
    return (
        tokens.some((t) => t.kind === 'emote') &&
        tokens.every((t) => t.kind === 'emote' || (t.kind === 'text' && t.text.trim() === ''))
    );
}

/** Where a share card leads (null: a lobby, joined through its own action). */
export function shareCardLink(att: MessageAttachment): string | null {
    if (!att.available) return null;
    if (att.kind === 'scene' && att.scene) {
        const t = att.time && att.time > 0 ? `?t=${Math.floor(att.time)}` : '';
        return `/watch/${att.scene.id}${t}`;
    }
    if (att.kind === 'marker' && att.marker) {
        const m = att.marker;
        const base = `/watch/${m.scene_id}?t=${m.start}`;
        return m.end != null && m.end > m.start ? `${base}&end=${m.end}` : base;
    }
    if (att.kind === 'playlist' && att.playlist) return `/playlists/${att.playlist.uuid}`;
    return null;
}

const KIND_LABELS: Record<string, string> = {
    scene: 'Video',
    marker: 'Marker',
    playlist: 'Playlist',
    lobby: 'Watch together',
};

export interface ShareCardInfo {
    kindLabel: string;
    icon: string;
    title: string;
    subtitle: string;
    thumbnail: string | null;
    // a badge over the thumbnail (duration, range, count)
    badge: string;
}

/** What a share card shows (available cards only). */
export function shareCardInfo(att: MessageAttachment): ShareCardInfo {
    const kindLabel = KIND_LABELS[att.kind] ?? 'Share';
    if (att.kind === 'scene' && att.scene) {
        const at = att.time && att.time > 0 ? `From ${formatClock(att.time)}` : 'Video';
        return {
            kindLabel,
            icon: 'heroicons:film',
            title: att.scene.title || 'Untitled',
            subtitle: at,
            thumbnail: att.scene.thumbnail_url ?? null,
            badge: att.scene.duration > 0 ? formatClock(att.scene.duration) : '',
        };
    }
    if (att.kind === 'marker' && att.marker) {
        const m = att.marker;
        const range =
            m.end != null && m.end > m.start
                ? `${formatClock(m.start)} - ${formatClock(m.end)}`
                : formatClock(m.start);
        return {
            kindLabel,
            icon: 'heroicons:bookmark',
            title: m.label || 'Marker',
            subtitle: m.scene_title || 'Untitled',
            thumbnail: m.thumbnail_url ?? null,
            badge: range,
        };
    }
    if (att.kind === 'playlist' && att.playlist) {
        const p = att.playlist;
        const unit = p.kind === 'markers' ? 'marker' : 'video';
        return {
            kindLabel,
            icon: 'heroicons:queue-list',
            title: p.name,
            subtitle: `By ${p.owner_username}`,
            thumbnail: p.thumbnail_url ?? null,
            badge: `${p.item_count} ${unit}${p.item_count === 1 ? '' : 's'}`,
        };
    }
    if (att.kind === 'lobby' && att.lobby) {
        const l = att.lobby;
        const playing = l.now_playing?.title ? `Playing ${l.now_playing.title}` : '';
        return {
            kindLabel,
            icon: 'heroicons:users',
            title: l.name,
            subtitle:
                playing || (l.host_username ? `Hosted by ${l.host_username}` : 'Watch together'),
            // the title (and so the thumbnail) only comes with scenes:view
            thumbnail: l.now_playing?.title ? `/thumbnails/${l.now_playing.scene_id}` : null,
            badge: `${l.online_count} online`,
        };
    }
    return {
        kindLabel,
        icon: 'heroicons:no-symbol',
        title: '',
        subtitle: '',
        thumbnail: null,
        badge: '',
    };
}

/** Why a share card is not available. */
export function unavailableShareText(att: MessageAttachment): string {
    const what =
        att.kind === 'scene'
            ? 'This video'
            : att.kind === 'marker'
              ? 'This marker'
              : att.kind === 'playlist'
                ? 'This playlist'
                : 'This lobby';
    switch (att.reason) {
        case 'trashed':
            return `${what} is in the trash`;
        case 'no_permission':
            return `${what} is not available to you`;
        case 'private':
            return `${what} is private`;
        case 'not_joinable':
            return 'This invite no longer works';
        default:
            return att.kind === 'lobby' ? 'This lobby is closed' : `${what} no longer exists`;
    }
}

/** Short text of the last message of a conversation. */
export function messagePreviewText(preview: MessagePreview | null, myId: number): string {
    if (!preview) return 'No message yet';
    const prefix = preview.sender_id === myId ? 'You: ' : '';
    if (preview.deleted) return `${prefix}Message deleted`;
    const body = preview.body.replace(/\s+/g, ' ').trim();
    if (body) return prefix + body;
    switch (preview.attachment_kind) {
        case 'scene':
            return `${prefix}Shared a video`;
        case 'marker':
            return `${prefix}Shared a marker`;
        case 'playlist':
            return `${prefix}Shared a playlist`;
        case 'lobby':
            return `${prefix}Sent a lobby invite`;
        default:
            return prefix.trim() || 'Message';
    }
}

/** Why I cannot write in a conversation. */
export function readOnlyText(reason: ReadOnlyReason | undefined, name: string): string {
    switch (reason) {
        case 'no_permission':
            return 'You are not allowed to send messages.';
        case 'deleted_user':
            return 'This account was deleted. The conversation is kept, read-only.';
        case 'policy':
            return `${name} does not accept messages from you anymore (their message settings, for example you are no longer friends). The history stays here.`;
        default:
            return '';
    }
}

/** Display name of the other participant. */
export function conversationTitle(conv: Pick<Conversation, 'other'>): string {
    if (!conv.other) return 'Deleted user';
    return conv.other.display_name || conv.other.username;
}

/** Path of the thread with a user. */
export function messagesPath(username: string): string {
    return `/messages/${encodeURIComponent(username)}`;
}

/** Most recent conversation first. */
export function sortConversations(list: readonly Conversation[]): Conversation[] {
    return [...list].sort(
        (a, b) =>
            new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime() || b.id - a.id,
    );
}

/**
 * A conversation after a new message: last message, date and unread count (a message of the
 * other side counts unless the thread is open).
 */
export function applyNewMessage(
    conv: Conversation,
    msg: Message,
    myId: number,
    viewing: boolean,
): Conversation {
    if (conv.last_message && conv.last_message.id >= msg.id) return conv;
    const fromOther = msg.sender_id !== myId;
    return {
        ...conv,
        last_message: {
            id: msg.id,
            sender_id: msg.sender_id,
            body: msg.body.slice(0, 280),
            attachment_kind: msg.attachment?.kind ?? null,
            deleted: msg.deleted,
            created_at: msg.created_at,
        },
        updated_at: msg.created_at,
        unread_count: fromOther && !viewing ? conv.unread_count + 1 : conv.unread_count,
        last_read_message_id: !fromOther || viewing ? msg.id : conv.last_read_message_id,
    };
}

/** Recipients of the share modal: recent conversations first, then friends, deduplicated. */
export interface ShareCandidate {
    id: number;
    username: string;
    display_name: string;
    avatar_version: number;
    can_message: boolean;
    recent: boolean;
}

export function shareCandidates(
    recent: readonly Conversation[],
    friends: readonly ShareCandidate[],
    query: string,
): ShareCandidate[] {
    const out: ShareCandidate[] = [];
    const seen = new Set<number>();
    const q = query.trim().toLowerCase();
    const matches = (c: { username: string; display_name: string }) =>
        !q || c.username.toLowerCase().includes(q) || c.display_name.toLowerCase().includes(q);
    for (const conv of recent) {
        const o = conv.other;
        if (!o || seen.has(o.id)) continue;
        const c: ShareCandidate = {
            id: o.id,
            username: o.username,
            display_name: o.display_name ?? '',
            avatar_version: o.avatar_version ?? 0,
            can_message: conv.can_send,
            recent: true,
        };
        if (!matches(c)) continue;
        seen.add(o.id);
        out.push(c);
    }
    for (const f of friends) {
        if (seen.has(f.id) || !matches(f)) continue;
        seen.add(f.id);
        out.push({ ...f, recent: false });
    }
    return out;
}

/** The items of a share, one target per item. */
export function shareItemKey(t: ShareTarget): string {
    return [t.kind, t.scene_id, t.marker_id, t.playlist_uuid, t.lobby_id, t.time].join(':');
}

/** Count shown on the header badge: empty under 1, "99+" over 99. */
export function unreadBadge(n: number): string {
    if (n < 1) return '';
    return n > 99 ? '99+' : String(n);
}

/**
 * Parses a time typed as seconds, m:ss or h:mm:ss (null when invalid). Minutes and seconds
 * after the first field must be below 60.
 */
export function parseClockInput(value: string): number | null {
    const v = value.trim();
    if (!v) return null;
    if (/^\d+$/.test(v)) return parseInt(v, 10);
    const parts = v.split(':');
    if (parts.length < 2 || parts.length > 3 || parts.some((p) => !/^\d+$/.test(p))) return null;
    const nums = parts.map((p) => parseInt(p, 10));
    if (nums.slice(1).some((n) => n >= 60)) return null;
    return nums.reduce((acc, n) => acc * 60 + n, 0);
}

/** The Share dialog subject of a marker (marker cards and rows). */
export function markerShareSubject(m: {
    id: number;
    scene_id: number;
    label: string;
    timestamp: number;
    end_timestamp?: number | null;
    scene_title?: string;
}): ShareSubject {
    return {
        kind: 'marker',
        markerId: m.id,
        sceneId: m.scene_id,
        label: m.label,
        start: m.timestamp,
        end: m.end_timestamp ?? null,
        sceneTitle: m.scene_title,
    };
}
