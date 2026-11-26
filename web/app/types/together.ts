// Goon2gether: synchronized watch lobbies (contract: docs/GOON2GETHER.md, protocol v1).

export type LobbyVisibility = 'public' | 'private';

export interface TogetherUserRef {
    id: number;
    username: string;
}

/** Lobby summary of the REST API (lobby browser, create, join). */
export interface LobbySummary {
    id: string;
    name: string;
    visibility: LobbyVisibility;
    host: TogetherUserRef | null;
    online_count: number;
    queue_length: number;
    now_playing: { scene_id: number; title?: string; label: string | null } | null;
    control_open: boolean;
    queue_open: boolean;
    wait_for_all: boolean;
    max_members: number;
    is_member: boolean;
    invite_code?: string;
    created_at: string;
}

/** A scene (or a range of it) to queue. */
export interface QueueItemInput {
    scene_id: number;
    start?: number | null;
    end?: number | null;
    label?: string | null;
}

export interface CreateLobbyInput {
    name?: string;
    visibility?: LobbyVisibility;
    control_open?: boolean;
    queue_open?: boolean;
    wait_for_all?: boolean;
    max_members?: number;
    items?: QueueItemInput[];
}

export interface Emote {
    id: number;
    name: string;
    url: string;
    width: number;
    height: number;
    animated: boolean;
}

/** Lobby settings as seen in the room. */
export interface LobbyInfo {
    id: string;
    name: string;
    visibility: LobbyVisibility;
    host_id: number | null;
    control_open: boolean;
    queue_open: boolean;
    wait_for_all: boolean;
    max_members: number;
    invite_code: string;
}

export interface LobbyMember {
    user_id: number;
    username: string;
    host: boolean;
    control: boolean;
    online: boolean;
    connections: number;
    buffering: boolean;
    joined_at: string;
}

export interface QueueItem {
    id: number;
    scene_id: number;
    title: string;
    duration: number;
    start: number | null;
    end: number | null;
    label: string | null;
    added_by: TogetherUserRef | null;
}

/** Server-authoritative playback state. */
export interface PlaybackState {
    item_id: number | null;
    playing: boolean;
    hold: boolean;
    /** seconds, at server_time */
    position: number;
    /** Unix ms of the server clock */
    server_time: number;
    rate: number;
    seq: number;
}

export interface ChatMessage {
    id: number;
    user_id: number | null;
    username: string | null;
    text: string;
    at: number;
    system: boolean;
}

export interface TogetherConfig {
    drift_threshold: number;
    drift_nudge: boolean;
    hold_timeout_ms: number;
    chat_max_length: number;
    end_grace: number;
}

export interface WelcomeMessage {
    t: 'welcome';
    v: number;
    server_time: number;
    you: { user_id: number; username: string; moderate: boolean };
    config: TogetherConfig;
    lobby: LobbyInfo;
    members: LobbyMember[];
    queue: QueueItem[];
    suggestions: QueueItem[];
    state: PlaybackState;
    chat: ChatMessage[];
}

export type ByeReason = 'kicked' | 'banned' | 'closed' | 'forbidden' | 'left';

export type ServerMessage =
    | WelcomeMessage
    | { t: 'state'; state: PlaybackState }
    | { t: 'queue'; queue: QueueItem[]; suggestions: QueueItem[] }
    | { t: 'members'; members: LobbyMember[] }
    | { t: 'lobby'; lobby: LobbyInfo }
    | { t: 'chat'; message: ChatMessage }
    | { t: 'chat_deleted'; id: number }
    | { t: 'reaction'; user_id: number; username: string; emote: string }
    | { t: 'emotes' }
    | { t: 'pong'; id: number; c: number; s: number }
    | { t: 'error'; code: string; message: string; ref?: string }
    | { t: 'bye'; reason: ByeReason };

export interface LobbySettingsInput {
    name?: string;
    visibility?: LobbyVisibility;
    control_open?: boolean;
    queue_open?: boolean;
    wait_for_all?: boolean;
    max_members?: number;
}

export type ClientMessage =
    | { t: 'ping'; id: number; c: number }
    | { t: 'play'; position?: number }
    | { t: 'pause'; position?: number }
    | { t: 'seek'; position: number }
    | { t: 'rate'; rate: number }
    | { t: 'skip' }
    | { t: 'play_item'; item_id: number }
    | { t: 'ended'; item_id: number }
    | { t: 'status'; buffering: boolean; seq: number; item_id: number | null }
    // play_now (control): the items go first and the first one plays at once
    | { t: 'queue_add'; items: QueueItemInput[]; play_now?: boolean }
    | { t: 'queue_remove'; item_id: number }
    | { t: 'queue_move'; item_id: number; index: number }
    | { t: 'suggestion_accept'; item_id: number }
    | { t: 'suggestion_reject'; item_id: number }
    | { t: 'chat'; text: string }
    | { t: 'chat_delete'; id: number }
    | { t: 'reaction'; emote: string }
    | ({ t: 'settings' } & LobbySettingsInput)
    | { t: 'grant_control'; user_id: number }
    | { t: 'revoke_control'; user_id: number }
    | { t: 'transfer_host'; user_id: number }
    | { t: 'kick'; user_id: number; ban?: boolean };

/** Connection status of the room socket. */
export type SocketStatus = 'connecting' | 'open' | 'reconnecting' | 'closed';
