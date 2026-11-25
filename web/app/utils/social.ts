// Pure helpers of the social features (docs/SOCIAL.md). Tests in web/tests/social.test.ts.
import type { PrivacyItem, Relationship } from '~/types/social';

/** Social SSE events, all targeted at the signed-in user (also listed in sse-worker.js). */
export const SOCIAL_SSE_EVENTS = [
    'social:friend_request',
    'social:friend_request_cancelled',
    'social:friend_request_accepted',
    'social:followed',
    'social:friend_requests_changed',
] as const;

export type SocialSseEvent = (typeof SOCIAL_SSE_EVENTS)[number];

export interface RelationshipBadge {
    key: 'friend' | 'mutual' | 'follows_you' | 'request_received' | 'request_sent';
    label: string;
}

/**
 * Badges shown next to a user: friend, mutual (only when not friends), follows you (only when
 * not mutual), and the pending request in either direction.
 */
export function relationshipBadges(rel: Relationship): RelationshipBadge[] {
    if (rel.self) return [];
    const out: RelationshipBadge[] = [];
    if (rel.friend) out.push({ key: 'friend', label: 'Friend' });
    if (rel.mutual && !rel.friend) out.push({ key: 'mutual', label: 'Mutual' });
    if (rel.followed_by && !rel.mutual) out.push({ key: 'follows_you', label: 'Follows you' });
    if (rel.request_received) out.push({ key: 'request_received', label: 'Wants to be friends' });
    if (rel.request_sent) out.push({ key: 'request_sent', label: 'Request sent' });
    return out;
}

/** The friend action available toward a user. */
export type FriendAction = 'none' | 'add' | 'cancel' | 'respond' | 'remove';

export function friendAction(rel: Relationship): FriendAction {
    if (rel.self) return 'none';
    if (rel.friend) return 'remove';
    if (rel.request_received) return 'respond';
    if (rel.request_sent) return 'cancel';
    if (rel.can_send_friend_request) return 'add';
    return 'none';
}

export interface PrivacyGroup {
    group: string;
    items: PrivacyItem[];
}

/** Groups the catalog items, keeping the catalog order of groups and items. */
export function groupPrivacyItems(items: PrivacyItem[]): PrivacyGroup[] {
    const groups: PrivacyGroup[] = [];
    for (const item of items) {
        let g = groups.find((x) => x.group === item.group);
        if (!g) {
            g = { group: item.group, items: [] };
            groups.push(g);
        }
        g.items.push(item);
    }
    return groups;
}

/** Count shown on a badge: empty under 1, "99+" over 99. */
export function badgeCount(n: number): string {
    if (n < 1) return '';
    return n > 99 ? '99+' : String(n);
}

/** Profile URL of a user. */
export function profilePath(username: string): string {
    return `/u/${encodeURIComponent(username)}`;
}
