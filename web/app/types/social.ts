// Social graph and privacy API shapes (contract: docs/SOCIAL.md).

/** Relation of the signed-in user toward another user. */
export interface Relationship {
    self: boolean;
    following: boolean;
    followed_by: boolean;
    mutual: boolean;
    friend: boolean;
    request_sent: boolean;
    request_received: boolean;
    can_send_friend_request: boolean;
}

export interface SocialUser {
    id: number;
    username: string;
    created_at: string;
}

/** A user in a social list; `since` is the date of the listed relation. */
export interface SocialUserEntry extends SocialUser {
    since: string;
    relationship: Relationship;
}

export interface SocialUserPage {
    data: SocialUserEntry[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}

/** GET /api/v1/social/users/:username. Counts are absent when hidden by privacy. */
export interface SocialProfile {
    user: SocialUser;
    relationship: Relationship;
    visibility: {
        profile: boolean;
        friends_list: boolean;
        followers_list: boolean;
    };
    counts: {
        friends?: number;
        followers?: number;
        following?: number;
    };
}

/** GET /api/v1/social/me */
export interface SocialSummary {
    incoming_requests: number;
    outgoing_requests: number;
    friends: number;
    followers: number;
    following: number;
}

export interface RelationshipChange {
    user_id: number;
    relationship: Relationship;
}

export interface FriendRequestSent extends RelationshipChange {
    status: 'sent' | 'accepted';
}

export type SocialListKind = 'friends' | 'followers' | 'following';

export type Audience = 'public' | 'followers' | 'mutuals' | 'friends' | 'private';

export type FriendRequestPolicy = 'everyone' | 'mutuals' | 'nobody';

export interface PrivacyItem {
    key: string;
    label: string;
    description: string;
    group: string;
    default: Audience;
    audience: Audience;
    is_default: boolean;
}

export interface PrivacyOption<T extends string = string> {
    value: T;
    label: string;
    description: string;
}

/** GET/PUT /api/v1/social/privacy */
export interface PrivacySettings {
    items: PrivacyItem[];
    audiences: PrivacyOption<Audience>[];
    friend_request_policy: FriendRequestPolicy;
    friend_request_policies: PrivacyOption<FriendRequestPolicy>[];
}

/** PUT /api/v1/social/privacy body: an audience or 'default' per item. */
export interface PrivacyUpdate {
    items?: Record<string, Audience | 'default'>;
    friend_request_policy?: FriendRequestPolicy;
}

/** Data of the social SSE events (targeted at the signed-in user only). */
export interface SocialEventData {
    user?: { id: number; username: string };
    incoming_requests?: number;
}
