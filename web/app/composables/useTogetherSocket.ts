import type {
    ByeReason,
    ChatMessage,
    ClientMessage,
    HistoryEntry,
    LobbyInfo,
    LobbyMember,
    PlaybackState,
    QueueItem,
    ReactionMessage,
    ServerMessage,
    SocketStatus,
    TogetherConfig,
    WelcomeMessage,
} from '~/types/together';
import type { ClockSample } from '~/utils/together';

const PROTOCOL_VERSION = 4;
// Clock sync: a burst on connect, then a slow refresh (also keeps proxies busy)
const PING_BURST = 5;
const PING_BURST_INTERVAL_MS = 400;
const PING_INTERVAL_MS = 15000;
// No message for this long (pongs come every 15 s): the connection is dead
const SILENCE_TIMEOUT_MS = 45000;
// Chat lines kept on the client (the server sends its own history on connect)
const CHAT_KEPT = 200;

export type ReactionEvent = Omit<ReactionMessage, 't'>;

export interface TogetherError {
    code: string;
    message: string;
}

const DEFAULT_CONFIG: TogetherConfig = {
    drift_threshold: 0.75,
    drift_nudge: true,
    hold_timeout_ms: 10000,
    chat_max_length: 500,
    end_grace: 1.5,
    chat_max_reaction_kinds: 20,
    chat_max_reactions_per_user: 10,
    history_max: 100,
};

/**
 * WebSocket of a Goon2gether room (protocol v4, docs/GOON2GETHER.md): reactive lobby state
 * resynced from the `welcome` snapshot on every (re)connect, clock offset from ping/pong,
 * reconnect with backoff unless the close is final (kicked, banned, closed, full).
 */
export const useTogetherSocket = (lobbyId: Ref<string>) => {
    const { load: loadEmotes } = useTogetherEmotes();

    const status = ref<SocketStatus>('connecting');
    const finalReason = ref<string | null>(null);
    // reason of the server's bye (closed for inactivity, kicked...), null otherwise
    const byeReason = ref<ByeReason | null>(null);
    const connectedOnce = ref(false);
    // the lobby closes at this time (server clock, ms) for inactivity unless something happens
    const closingAt = ref<number | null>(null);
    // join anonymously when connecting to a public lobby the user is not a member of
    let anonymous = false;

    const you = ref<WelcomeMessage['you'] | null>(null);
    const config = ref<TogetherConfig>({ ...DEFAULT_CONFIG });
    const lobby = ref<LobbyInfo | null>(null);
    const members = ref<LobbyMember[]>([]);
    const queue = ref<QueueItem[]>([]);
    const suggestions = ref<QueueItem[]>([]);
    const state = ref<PlaybackState | null>(null);
    const chat = ref<ChatMessage[]>([]);
    // played items, newest first
    const history = ref<HistoryEntry[]>([]);
    const lastError = ref<TogetherError | null>(null);

    // Clock offset: server time - client time (ms)
    let samples: ClockSample[] = [];
    const clockOffset = ref(0);
    const serverNow = () => Date.now() + clockOffset.value;

    const reactionHandlers = new Set<(r: ReactionEvent) => void>();
    const onReaction = (fn: (r: ReactionEvent) => void) => {
        reactionHandlers.add(fn);
        return () => reactionHandlers.delete(fn);
    };

    let ws: WebSocket | null = null;
    let attempt = 0;
    let pingId = 0;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let pingTimer: ReturnType<typeof setTimeout> | null = null;
    let silenceTimer: ReturnType<typeof setInterval> | null = null;
    let lastMessageAt = 0;
    let disposed = false;
    let errorTimer: ReturnType<typeof setTimeout> | null = null;

    const clearTimers = () => {
        if (reconnectTimer) clearTimeout(reconnectTimer);
        if (pingTimer) clearTimeout(pingTimer);
        if (silenceTimer) clearInterval(silenceTimer);
        reconnectTimer = pingTimer = silenceTimer = null;
    };

    const send = (msg: ClientMessage): boolean => {
        if (!ws || ws.readyState !== WebSocket.OPEN) return false;
        ws.send(JSON.stringify(msg));
        return true;
    };

    const sendPing = () => send({ t: 'ping', id: ++pingId, c: Date.now() });

    const schedulePings = (left: number) => {
        pingTimer = setTimeout(
            () => {
                sendPing();
                schedulePings(Math.max(0, left - 1));
            },
            left > 0 ? PING_BURST_INTERVAL_MS : PING_INTERVAL_MS,
        );
    };

    const showError = (err: TogetherError) => {
        lastError.value = err;
        if (errorTimer) clearTimeout(errorTimer);
        errorTimer = setTimeout(() => {
            lastError.value = null;
        }, 4000);
    };

    const pushChat = (message: ChatMessage) => {
        const next = [...chat.value, message];
        chat.value = next.length > CHAT_KEPT ? next.slice(next.length - CHAT_KEPT) : next;
    };

    const handle = (msg: ServerMessage) => {
        switch (msg.t) {
            case 'welcome':
                you.value = msg.you;
                config.value = { ...DEFAULT_CONFIG, ...msg.config };
                lobby.value = msg.lobby;
                members.value = msg.members ?? [];
                queue.value = msg.queue ?? [];
                suggestions.value = msg.suggestions ?? [];
                state.value = msg.state;
                chat.value = msg.chat ?? [];
                history.value = msg.history ?? [];
                closingAt.value = msg.closing_at ?? null;
                // Rough offset until the pongs refine it
                if (samples.length === 0) clockOffset.value = msg.server_time - Date.now();
                connectedOnce.value = true;
                status.value = 'open';
                attempt = 0;
                break;
            case 'state':
                if (!state.value || msg.state.seq >= state.value.seq) state.value = msg.state;
                break;
            case 'queue':
                queue.value = msg.queue ?? [];
                suggestions.value = msg.suggestions ?? [];
                break;
            case 'members':
                members.value = msg.members ?? [];
                break;
            case 'lobby':
                lobby.value = msg.lobby;
                break;
            case 'chat':
                pushChat(msg.message);
                break;
            case 'chat_deleted':
                chat.value = chat.value.filter((m) => m.id !== msg.id);
                break;
            case 'chat_reactions':
                chat.value = chat.value.map((m) =>
                    m.id === msg.id ? { ...m, reactions: msg.reactions ?? [] } : m,
                );
                break;
            case 'history_add':
                history.value = pushHistoryEntry(
                    history.value,
                    msg.entry,
                    config.value.history_max,
                );
                break;
            case 'reaction':
                for (const fn of reactionHandlers) fn(msg);
                break;
            case 'closing':
                closingAt.value = msg.at ?? null;
                break;
            case 'emotes':
                loadEmotes(true);
                break;
            case 'pong': {
                samples = pushClockSample(samples, clockSample(msg.c, msg.s, Date.now()));
                clockOffset.value = bestClockOffset(samples);
                break;
            }
            case 'error':
                showError({ code: msg.code, message: msg.message });
                break;
            case 'bye':
                byeReason.value = msg.reason;
                closingAt.value = null;
                finalReason.value = byeReasonText(msg.reason);
                break;
        }
    };

    const scheduleReconnect = () => {
        if (disposed) return;
        status.value = 'reconnecting';
        const delay = reconnectDelay(attempt++);
        reconnectTimer = setTimeout(connect, delay);
    };

    function connect() {
        clearTimers();
        if (disposed || !lobbyId.value) return;
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${proto}//${location.host}/api/v1/together/lobbies/${encodeURIComponent(
            lobbyId.value,
        )}/ws?v=${PROTOCOL_VERSION}${anonymous ? '&anonymous=1' : ''}`;
        status.value = connectedOnce.value ? 'reconnecting' : 'connecting';
        const socket = new WebSocket(url);
        ws = socket;

        socket.onopen = () => {
            lastMessageAt = Date.now();
            sendPing();
            schedulePings(PING_BURST - 1);
            silenceTimer = setInterval(() => {
                if (Date.now() - lastMessageAt > SILENCE_TIMEOUT_MS) socket.close(4000, 'silent');
            }, 5000);
        };
        socket.onmessage = (ev: MessageEvent) => {
            lastMessageAt = Date.now();
            if (typeof ev.data !== 'string') return;
            try {
                handle(JSON.parse(ev.data) as ServerMessage);
            } catch {
                // A malformed frame is ignored, the next state message corrects things
            }
        };
        socket.onclose = (ev: CloseEvent) => {
            if (ws !== socket) return;
            ws = null;
            clearTimers();
            if (disposed) return;
            if (!shouldReconnect(ev.code) || finalReason.value) {
                finalReason.value = finalReason.value ?? closeReasonText(ev.code);
                status.value = 'closed';
                return;
            }
            scheduleReconnect();
        };
    }

    const disconnect = () => {
        disposed = true;
        clearTimers();
        if (errorTimer) clearTimeout(errorTimer);
        if (ws) {
            const socket = ws;
            ws = null;
            socket.close(1000, 'leaving');
        }
        status.value = 'closed';
    };

    // Reconnect at once when the tab comes back online
    const onOnline = () => {
        if (status.value === 'reconnecting') {
            if (reconnectTimer) clearTimeout(reconnectTimer);
            connect();
        }
    };

    /** Connects; `asAnonymous` joins a public lobby anonymously if not a member yet. */
    const start = (asAnonymous = false) => {
        anonymous = asAnonymous;
        disposed = false;
        finalReason.value = null;
        byeReason.value = null;
        closingAt.value = null;
        connectedOnce.value = false;
        samples = [];
        attempt = 0;
        window.addEventListener('online', onOnline);
        connect();
    };

    onScopeDispose(() => {
        window.removeEventListener('online', onOnline);
        disconnect();
    });

    return {
        status,
        finalReason,
        byeReason,
        closingAt,
        connectedOnce,

        you,
        config,
        lobby,
        members,
        queue,
        suggestions,
        state,
        chat,
        history,
        lastError,
        clockOffset,
        serverNow,
        send,
        onReaction,
        start,
        disconnect,
        showError,
    };
};

export type TogetherSocket = ReturnType<typeof useTogetherSocket>;
