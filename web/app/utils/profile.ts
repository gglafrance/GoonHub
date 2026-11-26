// Pure helpers of the profile pages (docs/PROFILES.md). Tests in web/tests/profile.test.ts.
import type { ImageCrop, ProfileLayoutBlock, ProfileListKind } from '~/types/profile';

export type AvatarSize = 'sm' | 'md' | 'lg';

/** URL of an avatar, or null when the user has none (version 0: initials). */
export function avatarUrl(
    userId: number | undefined,
    version: number | undefined,
    size: AvatarSize = 'sm',
): string | null {
    if (!userId || !version || version <= 0) return null;
    return `/api/v1/profiles/avatar/${userId}/${size}?v=${version}`;
}

/** Initials shown without an avatar: first letters of up to two words. */
export function avatarInitials(name: string): string {
    const words = name
        .trim()
        .split(/[\s._-]+/)
        .filter(Boolean);
    if (words.length === 0) return '?';
    const first = Array.from(words[0]!)[0] ?? '?';
    const second = words.length > 1 ? (Array.from(words[1]!)[0] ?? '') : '';
    return (first + second).toUpperCase();
}

/** The name to show for a user: their display name, else their username. */
export function displayNameOf(user: { username: string; display_name?: string }): string {
    return user.display_name?.trim() || user.username;
}

export interface BioSegment {
    text: string;
    href?: string;
}

const URL_PATTERN = /\b(?:https?:\/\/|www\.)[^\s<>"']+/gi;
const TRAILING_PUNCTUATION = /[.,;:!?)\]}]+$/;

/**
 * Splits a bio into text and link segments. Only http(s) URLs (and www. ones, made https)
 * become links; everything is rendered as text nodes, never as HTML.
 */
export function bioSegments(bio: string): BioSegment[] {
    const out: BioSegment[] = [];
    let last = 0;
    for (const match of bio.matchAll(URL_PATTERN)) {
        let raw = match[0];
        const trailing = raw.match(TRAILING_PUNCTUATION)?.[0] ?? '';
        raw = raw.slice(0, raw.length - trailing.length);
        const start = match.index ?? 0;
        if (start > last) out.push({ text: bio.slice(last, start) });
        const href = raw.toLowerCase().startsWith('www.') ? `https://${raw}` : raw;
        let safe: string | undefined;
        try {
            const url = new URL(href);
            if (url.protocol === 'http:' || url.protocol === 'https:') safe = url.href;
        } catch {
            safe = undefined;
        }
        out.push(safe ? { text: raw, href: safe } : { text: raw });
        last = start + raw.length;
    }
    if (last < bio.length) out.push({ text: bio.slice(last) });
    return out;
}

/** Watch time in a short form: "3h 20m", "45m", "2d 4h". */
export function formatWatchTime(seconds: number): string {
    const s = Math.max(0, Math.floor(seconds));
    const days = Math.floor(s / 86400);
    const hours = Math.floor((s % 86400) / 3600);
    const minutes = Math.floor((s % 3600) / 60);
    if (days > 0) return hours ? `${days}d ${hours}h` : `${days}d`;
    if (hours > 0) return minutes ? `${hours}h ${minutes}m` : `${hours}h`;
    return `${minutes}m`;
}

/** Returns a copy of the list with the item at `from` moved to `to`. */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
    const out = list.slice();
    if (from < 0 || from >= out.length || to < 0 || to >= out.length || from === to) return out;
    const [item] = out.splice(from, 1);
    out.splice(to, 0, item!);
    return out;
}

/** The blocks of a layout that share a privacy item with a block (other than itself). */
export function blocksSharingPrivacy(
    layout: ProfileLayoutBlock[],
    privacyOf: (type: string) => string | undefined,
    type: string,
): string[] {
    const item = privacyOf(type);
    if (!item) return [];
    return layout.filter((b) => b.type !== type && privacyOf(b.type) === item).map((b) => b.type);
}

/**
 * Crop rectangle (source pixels) of an image shown in a frame of frameW x frameH, scaled by
 * `scale` and moved by (offsetX, offsetY) display pixels (offset of the image's top-left
 * corner from the frame's, <= 0 when the image covers the frame).
 */
export function cropFromView(
    natural: { width: number; height: number },
    frame: { width: number; height: number },
    scale: number,
    offsetX: number,
    offsetY: number,
): ImageCrop {
    const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi);
    const w = clamp(Math.round(frame.width / scale), 1, natural.width);
    const h = clamp(Math.round(frame.height / scale), 1, natural.height);
    const x = clamp(Math.round(-offsetX / scale), 0, natural.width - w);
    const y = clamp(Math.round(-offsetY / scale), 0, natural.height - h);
    return { x, y, w, h };
}

/** Clamps an offset so that the scaled image keeps covering the frame. */
export function clampOffset(offset: number, scaledSize: number, frameSize: number): number {
    const min = Math.min(0, frameSize - scaledSize);
    return Math.min(0, Math.max(min, offset));
}

/** Icon of each block type. */
export const PROFILE_BLOCK_ICONS: Record<string, string> = {
    favorite_scenes: 'heroicons:star',
    favorite_markers: 'heroicons:bookmark',
    recent_watches: 'heroicons:clock',
    most_watched: 'heroicons:fire',
    recent_markers: 'heroicons:bookmark-square',
    liked_scenes: 'heroicons:heart',
    stats: 'heroicons:chart-bar',
    ratings: 'heroicons:star',
    jizz: 'fluent-emoji-high-contrast:sweat-droplets',
    activity_calendar: 'heroicons:calendar-days',
    playlists: 'heroicons:queue-list',
    friends: 'heroicons:user-group',
};

/** The full list a block opens with its "See all" link. */
export const PROFILE_BLOCK_LISTS: Record<string, ProfileListKind> = {
    liked_scenes: 'likes',
    ratings: 'ratings',
    jizz: 'jizz',
};

/** Path of a profile tab: the profile itself, or one of its lists (with an optional query). */
export function profileTabPath(
    username: string,
    list?: ProfileListKind,
    query?: Record<string, string>,
): string {
    const base = `/u/${encodeURIComponent(username)}${list ? `/${list}` : ''}`;
    const qs = query ? new URLSearchParams(query).toString() : '';
    return qs ? `${base}?${qs}` : base;
}
