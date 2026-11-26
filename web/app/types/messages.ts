// Messages between users and shares (contract: docs/MESSAGES.md).

export type MessagePolicy = 'everyone' | 'mutuals' | 'friends' | 'nobody';

export type AttachmentKind = 'scene' | 'marker' | 'playlist' | 'lobby';

export type ReadOnlyReason = 'no_permission' | 'deleted_user' | 'policy';

export type ShareReason = 'not_found' | 'trashed' | 'no_permission' | 'private' | 'not_joinable';

/** Card of a participant. */
export interface MessageUser {
    id: number;
    username: string;
    display_name: string;
    avatar_version: number;
}

export interface ShareSceneCard {
    id: number;
    title: string;
    duration: number;
    thumbnail_url?: string;
}

export interface ShareMarkerCard {
    id: number;
    scene_id: number;
    scene_title: string;
    scene_duration: number;
    start: number;
    end: number | null;
    label: string;
    color: string;
    owner_username: string;
    thumbnail_url?: string;
}

export interface SharePlaylistCard {
    uuid: string;
    name: string;
    kind: 'scenes' | 'markers';
    visibility: string;
    owner_username: string;
    item_count: number;
    thumbnail_url?: string;
}

export interface LobbyInviteCard {
    id: string;
    name: string;
    visibility: 'public' | 'private';
    online_count: number;
    host_username?: string;
    now_playing: { scene_id: number; title?: string } | null;
    is_member: boolean;
    joinable: boolean;
}

/** The share of a message, resolved for me (only kind and reason when not available). */
export interface MessageAttachment {
    kind: AttachmentKind;
    available: boolean;
    reason?: ShareReason;
    time?: number;
    scene?: ShareSceneCard;
    marker?: ShareMarkerCard;
    playlist?: SharePlaylistCard;
    lobby?: LobbyInviteCard;
}

/** A user who reacted to a message (a participant). */
export interface ReactionUser {
    id: number;
    username: string;
    display_name: string;
}

/**
 * One reaction key of a message: an emoji or `:name:` of a server emote, how many reacted and
 * who (in reaction order). The keys come in the order of their first reaction.
 */
export interface MessageReaction {
    emoji: string;
    count: number;
    users: ReactionUser[];
}

export interface Message {
    id: number;
    conversation_id: number;
    // null: deleted user
    sender_id: number | null;
    body: string;
    attachment: MessageAttachment | null;
    // empty for a deleted message (older payloads may omit it)
    reactions?: MessageReaction[];
    edited_at: string | null;
    deleted: boolean;
    created_at: string;
}

export interface MessagePreview {
    id: number;
    sender_id: number | null;
    body: string;
    attachment_kind: AttachmentKind | null;
    deleted: boolean;
    created_at: string;
}

export interface Conversation {
    id: number;
    kind: 'direct';
    // null: deleted user
    other: MessageUser | null;
    last_message: MessagePreview | null;
    unread_count: number;
    muted: boolean;
    last_read_message_id: number | null;
    other_last_read_message_id: number | null;
    can_send: boolean;
    read_only_reason?: ReadOnlyReason;
    updated_at: string;
}

export interface ConversationPage {
    data: Conversation[];
    pagination: { page: number; limit: number; total_items: number; total_pages: number };
}

/** GET /messages/with/:username */
export interface DirectThread {
    user: MessageUser;
    conversation: Conversation | null;
    can_send: boolean;
    read_only_reason?: ReadOnlyReason;
}

export interface MessageHistory {
    messages: Message[];
    has_more: boolean;
}

/** What a message shares (request). */
export interface ShareTarget {
    kind: AttachmentKind;
    scene_id?: number;
    time?: number;
    marker_id?: number;
    playlist_uuid?: string;
    lobby_id?: string;
}

export interface SendMessageBody {
    body: string;
    attachment?: ShareTarget | null;
}

export interface ShareResult {
    user_id: number;
    ok: boolean;
    conversation_id?: number;
    code?: string;
    error?: string;
}

export interface MessagePolicyOption {
    value: MessagePolicy;
    label: string;
    description: string;
}

export interface MessageSettings {
    policy: MessagePolicy;
    toasts: boolean;
    policies: MessagePolicyOption[];
    max_length: number;
    typing_interval_ms: number;
    max_reaction_kinds: number;
    max_reactions_per_user: number;
}

export interface UnreadCounts {
    messages: number;
    conversations: number;
}

// SSE payloads (chat:*)
export interface ChatMessageEvent {
    conversation_id: number;
    message: Message;
}

export interface ChatReadEvent {
    conversation_id: number;
    user_id: number;
    last_read_message_id: number;
}

export interface ChatTypingEvent {
    conversation_id: number;
    user_id: number;
}

export interface ChatConversationEvent {
    conversation_id: number;
}

/** chat:reaction, also the answer of the reaction routes: the whole list of the message. */
export interface ChatReactionEvent {
    conversation_id: number;
    message_id: number;
    reactions: MessageReaction[];
}
