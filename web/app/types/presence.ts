// Presence API shapes (contract: docs/PRESENCE.md).

export type PresenceStatus = 'online' | 'idle' | 'offline';

/** How a contact is listed: watching (online with an activity), online, idle, offline. */
export type PresenceGroupKey = 'watching' | 'online' | 'idle' | 'offline';

/** Scene of a watching_scene activity: title and thumbnail only. */
export interface PresenceScene {
    id: number;
    title: string;
    duration: number;
    thumbnail_url?: string;
}

/**
 * What a user is doing. `scene`, `position`, `position_at` and `paused` are absent when the
 * viewer may not see the scene ("watching something"). Other features add their `type`.
 */
export interface PresenceActivity {
    type: 'watching_scene' | (string & {});
    started_at: string;
    scene?: PresenceScene;
    position?: number;
    position_at?: string;
    paused?: boolean;
}

/**
 * Presence of a user as the signed-in user may see it (snapshot entries, presence:update
 * events). `visible: false`: the presence of that user is hidden (or no longer visible).
 */
export interface PresenceView {
    user: { id: number; username: string };
    visible: boolean;
    status?: PresenceStatus;
    last_seen_at?: string;
    activity?: PresenceActivity;
    seq: number;
}

/** GET /api/v1/social/presence */
export interface PresenceSnapshot {
    users: PresenceView[];
    invisible: boolean;
    heartbeat_interval_seconds: number;
    activity_interval_seconds: number;
}

/** PUT /api/v1/social/presence/activity body */
export interface PresenceActivityReport {
    tab_id: string;
    type: 'watching_scene';
    scene_id: number;
    position: number;
    paused: boolean;
}

export interface PresenceSettings {
    invisible: boolean;
}
