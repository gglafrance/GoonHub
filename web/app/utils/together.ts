// Pure Goon2gether logic (docs/GOON2GETHER.md "Sync algorithm"): clock offset, expected
// position, drift correction, reconnect backoff, chat emote tokens. Tests in
// web/tests/together*.test.ts.
import type {
    AutoNextMode,
    ByeReason,
    LobbyMember,
    MemberTarget,
    PlaybackState,
    QueueItem,
} from '~/types/together';

/** Broadcast count of the public lobbies (nav badge, also listed in sse-worker.js). */
export const TOGETHER_PUBLIC_COUNT_EVENT = 'together:public_count';

/** Goon2gether SSE events dispatched to the onSSEEvent listeners. */
export const TOGETHER_SSE_EVENTS = [TOGETHER_PUBLIC_COUNT_EVENT] as const;

// ---------------------------------------------------------------------------
// Clock offset
// ---------------------------------------------------------------------------

export interface ClockSample {
    rtt: number;
    offset: number;
}

/** Samples kept: the one with the lowest RTT among them gives the offset. */
export const CLOCK_SAMPLES_KEPT = 8;

/**
 * One ping/pong exchange: `c` client send time, `s` server time, `r` client receive time
 * (all ms). The server time is assumed to be read half way through the round trip.
 */
export function clockSample(c: number, s: number, r: number): ClockSample {
    const rtt = Math.max(0, r - c);
    return { rtt, offset: s - (c + rtt / 2) };
}

/** Adds a sample, keeping the last CLOCK_SAMPLES_KEPT ones. */
export function pushClockSample(samples: ClockSample[], sample: ClockSample): ClockSample[] {
    const next = [...samples, sample];
    return next.length > CLOCK_SAMPLES_KEPT ? next.slice(next.length - CLOCK_SAMPLES_KEPT) : next;
}

/** Offset (server - client, ms) of the least delayed sample, 0 without samples. */
export function bestClockOffset(samples: readonly ClockSample[]): number {
    let best: ClockSample | null = null;
    for (const s of samples) {
        if (!best || s.rtt < best.rtt) best = s;
    }
    return best ? best.offset : 0;
}

// ---------------------------------------------------------------------------
// Expected position
// ---------------------------------------------------------------------------

/** End of an item in seconds: its range end, else the scene duration, else null (unknown). */
export function itemEnd(item: Pick<QueueItem, 'end' | 'duration'> | null | undefined) {
    if (!item) return null;
    if (item.end != null) return item.end;
    return item.duration > 0 ? item.duration : null;
}

/** The lobby moves forward (playing and not held). */
export function stateAdvances(state: Pick<PlaybackState, 'playing' | 'hold'>): boolean {
    return state.playing && !state.hold;
}

/**
 * Position (seconds) the lobby is at, at `serverNow` (ms, server clock), clamped to
 * [0, end] when the end is known.
 */
export function expectedPosition(
    state: Pick<PlaybackState, 'position' | 'server_time' | 'rate' | 'playing' | 'hold'>,
    serverNow: number,
    end: number | null = null,
): number {
    let pos = state.position;
    if (stateAdvances(state)) {
        pos += (Math.max(0, serverNow - state.server_time) / 1000) * state.rate;
    }
    if (pos < 0) pos = 0;
    if (end != null && pos > end) pos = end;
    return pos;
}

// ---------------------------------------------------------------------------
// Drift correction
// ---------------------------------------------------------------------------

/** Seek threshold floor while the scene is converted on the fly (a seek restarts ffmpeg). */
export const TRANSCODE_DRIFT_THRESHOLD = 2.5;
/** Drift under which no nudge starts. */
export const NUDGE_START = 0.1;
/** Drift under which a nudge stops. */
export const NUDGE_STOP = 0.05;
/** Largest relative rate change of a nudge. */
export const NUDGE_MAX = 0.05;

export type DriftAction =
    | { kind: 'seek'; position: number }
    | { kind: 'rate'; rate: number }
    | { kind: 'none' };

export interface DriftInput {
    /** player time (s) */
    current: number;
    /** expected position (s) */
    expected: number;
    /** lobby rate */
    rate: number;
    /** seconds over which the player seeks */
    threshold: number;
    nudge: boolean;
    transcoding: boolean;
    /** a nudge is running (then it stops under NUDGE_STOP rather than NUDGE_START) */
    nudging: boolean;
}

/**
 * What to do about the drift between the player and the lobby: seek when it is large, else
 * play slightly faster or slower (a nudge) until it is tiny, else play at the lobby rate.
 */
export function driftCorrection(input: DriftInput): DriftAction {
    const drift = input.current - input.expected;
    const threshold = input.transcoding
        ? Math.max(input.threshold, TRANSCODE_DRIFT_THRESHOLD)
        : input.threshold;
    if (Math.abs(drift) > threshold) return { kind: 'seek', position: input.expected };
    const limit = input.nudging ? NUDGE_STOP : NUDGE_START;
    if (input.nudge && Math.abs(drift) > limit) {
        const factor = Math.min(NUDGE_MAX, Math.max(-NUDGE_MAX, drift * 0.1));
        return { kind: 'rate', rate: input.rate * (1 - factor) };
    }
    return { kind: 'rate', rate: input.rate };
}

// ---------------------------------------------------------------------------
// Reconnect
// ---------------------------------------------------------------------------

/** Close codes after which the client must not reconnect (kicked, banned, closed, full). */
export const FINAL_CLOSE_CODES = [4001, 4003, 4004, 4005, 4009];

export function shouldReconnect(code: number): boolean {
    return !FINAL_CLOSE_CODES.includes(code);
}

/** Delay before reconnect attempt `attempt` (0-based): 1 s doubling, max 30 s, +-20% jitter. */
export function reconnectDelay(attempt: number, random: () => number = Math.random): number {
    const base = Math.min(30000, 1000 * 2 ** Math.max(0, attempt));
    const jitter = 1 + (random() * 2 - 1) * 0.2;
    return Math.round(Math.min(30000, base * jitter));
}

/** User-facing text of a final close. */
export function closeReasonText(code: number): string {
    switch (code) {
        case 4001:
            return 'You were kicked from this lobby.';
        case 4003:
            return 'You are not allowed in this lobby.';
        case 4004:
            return 'This lobby was closed.';
        case 4005:
            return 'You left this lobby.';
        case 4009:
            return 'This lobby is full, or you have too many tabs open in it.';
        default:
            return 'Disconnected.';
    }
}

// ---------------------------------------------------------------------------
// Chat emotes
// ---------------------------------------------------------------------------

export type ChatToken =
    | { kind: 'text'; text: string }
    | { kind: 'emote'; name: string; url: string };

const EMOTE_TOKEN = /:([A-Za-z0-9_]{2,32}):/g;

/**
 * Splits a chat message into text and emote tokens (`:name:` of a known emote, matched
 * without regard to case). Unknown `:words:` stay text. Rendered as text nodes and <img>.
 */
export function tokenizeChat(
    text: string,
    emotes: ReadonlyMap<string, { name: string; url: string }>,
): ChatToken[] {
    const tokens: ChatToken[] = [];
    let last = 0;
    const pushText = (s: string) => {
        if (!s) return;
        const prev = tokens[tokens.length - 1];
        if (prev && prev.kind === 'text') prev.text += s;
        else tokens.push({ kind: 'text', text: s });
    };
    for (const match of text.matchAll(EMOTE_TOKEN)) {
        const emote = emotes.get(match[1]!.toLowerCase());
        const index = match.index ?? 0;
        if (!emote) continue;
        pushText(text.slice(last, index));
        tokens.push({ kind: 'emote', name: emote.name, url: emote.url });
        last = index + match[0].length;
    }
    pushText(text.slice(last));
    return tokens;
}

/** Emote lookup by lower-case name. */
export function emoteMap<T extends { name: string }>(emotes: readonly T[]): Map<string, T> {
    return new Map(emotes.map((e) => [e.name.toLowerCase(), e]));
}

/**
 * The `:partial` being typed right before the caret, or null. At least one character after
 * the colon, and the colon starts a word.
 */
export function emoteQueryAt(text: string, caret: number): { start: number; query: string } | null {
    const before = text.slice(0, caret);
    const match = /(^|\s):([A-Za-z0-9_]{1,32})$/.exec(before);
    if (!match) return null;
    const query = match[2]!;
    return { start: caret - query.length - 1, query };
}

/** Emotes matching an autocomplete query: prefix matches first, then substring, by name. */
export function matchEmotes<T extends { name: string }>(
    emotes: readonly T[],
    query: string,
    limit = 8,
): T[] {
    const q = query.toLowerCase();
    const prefix: T[] = [];
    const inner: T[] = [];
    for (const e of emotes) {
        const n = e.name.toLowerCase();
        if (n.startsWith(q)) prefix.push(e);
        else if (n.includes(q)) inner.push(e);
    }
    const byName = (a: T, b: T) => a.name.localeCompare(b.name);
    return [...prefix.sort(byName), ...inner.sort(byName)].slice(0, limit);
}

/** Text after replacing the `:partial` at `start` (up to the caret) with `:name: `. */
export function completeEmote(
    text: string,
    start: number,
    caret: number,
    name: string,
): { text: string; caret: number } {
    const insert = `:${name}: `;
    return {
        text: text.slice(0, start) + insert + text.slice(caret),
        caret: start + insert.length,
    };
}

/** A reaction value is a custom emote (`:name:`) rather than an emoji. */
export function isCustomEmoteReaction(value: string): boolean {
    return /^:[A-Za-z0-9_]{2,32}:$/.test(value);
}

/** User-facing text of the server's bye. */
export function byeReasonText(reason: ByeReason): string {
    switch (reason) {
        case 'kicked':
            return closeReasonText(4001);
        case 'closed':
            return closeReasonText(4004);
        case 'left':
            return closeReasonText(4005);
        case 'inactive':
            return 'This lobby was closed for inactivity: nothing happened in it for a while.';
        default:
            return closeReasonText(4003);
    }
}

// ---------------------------------------------------------------------------
// Members (anonymous members have a null user_id for everyone but the moderators)
// ---------------------------------------------------------------------------

/** Stable key of a member in a list. */
export function memberKey(m: Pick<LobbyMember, 'user_id' | 'anon_number'>): string {
    return m.user_id != null ? `u${m.user_id}` : `a${m.anon_number ?? 0}`;
}

/** How a command names a member: anonymous members by their number. */
export function memberTarget(
    m: Pick<LobbyMember, 'user_id' | 'anonymous' | 'anon_number'>,
): MemberTarget | null {
    if (m.anonymous && m.anon_number != null) return { anon: m.anon_number };
    return m.user_id != null ? { user_id: m.user_id } : null;
}

/**
 * Whether a user reference (a member, a chat author, who added an item) is the signed-in
 * user: by user id, or by anonymous number when the user is anonymous in the lobby.
 */
export function isSelf(
    id: number | null | undefined,
    anon: number | null | undefined,
    you: { user_id: number; anon_number: number | null } | null | undefined,
): boolean {
    if (!you) return false;
    if (id != null && id === you.user_id) return true;
    return you.anon_number != null && anon != null && anon === you.anon_number;
}

// ---------------------------------------------------------------------------
// Auto-next
// ---------------------------------------------------------------------------

export const AUTO_NEXT_MODES: { value: AutoNextMode; label: string; hint: string; icon: string }[] =
    [
        {
            value: 'next',
            label: 'Play next',
            hint: 'The next queue item starts',
            icon: 'heroicons:forward',
        },
        {
            value: 'stop',
            label: 'Stop',
            hint: 'Pause at the end of each item',
            icon: 'heroicons:stop',
        },
        {
            value: 'repeat_one',
            label: 'Repeat item',
            hint: 'Loop the item playing (its range for a marker)',
            icon: 'heroicons:arrow-path-rounded-square',
        },
        {
            value: 'repeat_queue',
            label: 'Repeat queue',
            hint: 'Start over at the first item after the last one',
            icon: 'heroicons:arrow-path',
        },
    ];

/** The auto-next mode entry, `next` for an unknown value. */
export function autoNextMode(value: string | null | undefined) {
    return AUTO_NEXT_MODES.find((m) => m.value === value) ?? AUTO_NEXT_MODES[0]!;
}

// ---------------------------------------------------------------------------
// Misc
// ---------------------------------------------------------------------------

/** Invite link of a lobby code. */
export function inviteLink(origin: string, code: string): string {
    return `${origin.replace(/\/$/, '')}/together/join/${encodeURIComponent(code)}`;
}

/** m:ss or h:mm:ss. */
export function formatClock(seconds: number): string {
    const s = Math.max(0, Math.floor(seconds));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    const mm = h > 0 ? String(m).padStart(2, '0') : String(m);
    return `${h > 0 ? `${h}:` : ''}${mm}:${String(sec).padStart(2, '0')}`;
}

/** Moves the element at `from` to `to` (both clamped), returning a new array. */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
    const next = [...list];
    if (from < 0 || from >= next.length) return next;
    const [item] = next.splice(from, 1);
    const target = Math.max(0, Math.min(next.length, to));
    next.splice(target, 0, item as T);
    return next;
}

// ---------------------------------------------------------------------------
// Emote management
// ---------------------------------------------------------------------------

/** Emote names: 2 to 32 letters, digits or underscores (unique without regard to case). */
export function isValidEmoteName(name: string): boolean {
    return /^[A-Za-z0-9_]{2,32}$/.test(name);
}

/** Default emote name from a file name ("Pog Champ!.png" gives "Pog_Champ"). */
export function emoteNameFromFile(fileName: string): string {
    const base = fileName.replace(/\.[^.]*$/, '');
    return base
        .replace(/[^A-Za-z0-9_]+/g, '_')
        .replace(/^_+|_+$/g, '')
        .slice(0, 32);
}

export const EMOTE_TYPES = ['image/png', 'image/gif', 'image/webp'];
/** Client-side hint only: the server limit is goon2gether.emote_max_bytes. */
export const EMOTE_HINT_MAX_BYTES = 512 * 1024;

/** Why a file cannot be an emote, or null. */
export function emoteFileProblem(file: { type: string; size: number }): string | null {
    if (!EMOTE_TYPES.includes(file.type)) return 'Only PNG, GIF and WebP images';
    if (file.size > EMOTE_HINT_MAX_BYTES) return 'The image is too large (512 KB at most)';
    return null;
}
