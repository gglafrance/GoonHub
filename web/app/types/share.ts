export interface ShareLink {
    id: number;
    token: string;
    scene_id: number;
    user_id: number;
    share_type: 'public' | 'auth_required';
    expires_at: string | null;
    view_count: number;
    created_at: string;
    // the player starts here (seconds); end and label: a shared marker range (docs/MESSAGES.md)
    start?: number | null;
    end?: number | null;
    label?: string | null;
}

export interface ShareSceneData {
    id: number;
    title: string;
    description: string;
    duration: number;
    studio: string;
    tags: string[];
    actors: string[];
    created_at: string;
    release_date: string | null;
    playback?: import('~/types/scene').ScenePlayback;
}

export interface ResolvedShareLink {
    share_link: ShareLink;
    scene: ShareSceneData;
}

export interface ShareLinksResponse {
    share_links: ShareLink[];
    share_base_url: string;
}

/** Options of a new share link. */
export interface ShareLinkOptions {
    start?: number;
    marker_id?: number;
}

/** What a Share dialog shares (docs/MESSAGES.md, "Share dialog"). */
export type ShareSubject =
    // time: the player position when opened from the watch page (prefills "Start at")
    | { kind: 'scene'; sceneId: number; title?: string; duration?: number; time?: number }
    // several scenes (selection): sent to friends only
    | { kind: 'scenes'; sceneIds: number[] }
    | {
          kind: 'marker';
          markerId: number;
          sceneId: number;
          label: string;
          start: number;
          end: number | null;
          sceneTitle?: string;
      }
    | { kind: 'playlist'; uuid: string; name: string }
    | { kind: 'lobby'; lobbyId: string; name: string };
