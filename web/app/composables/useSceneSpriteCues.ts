import type { VttCue } from './useVttParser';

// Sprite cues per VTT URL, shared by every scene card. Scenes without sprites resolve to [].
const cueCache = new Map<string, Promise<VttCue[]>>();
const MAX_CACHED_SCENES = 500;

/** Loads (once) the sprite sheet cues of a scene, used by the scene card scrubber. */
export function loadSceneSpriteCues(sceneId: number, version?: string): Promise<VttCue[]> {
    const url = version ? `/vtt/${sceneId}?v=${version}` : `/vtt/${sceneId}`;
    const cached = cueCache.get(url);
    if (cached) return cached;

    const promise = fetch(url)
        .then((response) => (response.ok ? response.text() : ''))
        .then((text) => parseVttCues(text))
        .catch(() => [] as VttCue[]);

    if (cueCache.size >= MAX_CACHED_SCENES) {
        const oldest = cueCache.keys().next().value;
        if (oldest !== undefined) cueCache.delete(oldest);
    }
    cueCache.set(url, promise);
    return promise;
}

/** Finds the cue covering a time, falling back to the closest one. */
export function findSpriteCue(cues: VttCue[], time: number): VttCue | null {
    if (cues.length === 0) return null;
    const cue = cues.find((c) => time >= c.start && time < c.end);
    if (cue) return cue;
    return time < cues[0]!.start ? cues[0]! : cues[cues.length - 1]!;
}
