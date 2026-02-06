import type { VttCue } from './useVttParser';
import type { ExtractedColors } from './useColorExtractor';

interface ColorKeyframe {
    time: number;
    colors: ExtractedColors;
}

interface GlowStyle {
    [key: `--${string}`]: string | number;
}

const DEFAULT_COLORS: ExtractedColors = {
    primary: 'rgb(0, 0, 0)',
    secondary: 'rgb(0, 0, 0)',
    brightness: 1,
};

function parseRgb(rgb: string): [number, number, number] {
    const match = rgb.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
    if (!match) return [255, 77, 77];
    return [parseInt(match[1]!), parseInt(match[2]!), parseInt(match[3]!)];
}

function lerpColor(c1: string, c2: string, t: number): string {
    const [r1, g1, b1] = parseRgb(c1);
    const [r2, g2, b2] = parseRgb(c2);
    const r = Math.round(r1 + (r2 - r1) * t);
    const g = Math.round(g1 + (g2 - g1) * t);
    const b = Math.round(b1 + (b2 - b1) * t);
    return `rgb(${r}, ${g}, ${b})`;
}

function easeInOutQuad(t: number): number {
    return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
}

export function useAmbientGlow(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    playerRef: Ref<any>,
    vttCues: Ref<VttCue[]>,
    thumbnailUrl: Ref<string>,
    isPlaying: Ref<boolean>,
) {
    const { extractFromImage, extractFromImageRegion } = useColorExtractor();

    const keyframes = ref<ColorKeyframe[]>([]);
    const currentColors = ref<ExtractedColors>(DEFAULT_COLORS);
    const isProcessing = ref(false);
    const spriteCache = new Map<string, HTMLImageElement>();

    let animationId: number | null = null;
    let lastUpdateTime = 0;

    const glowStyle = computed<GlowStyle>(() => {
        const baseOpacity = 1;
        // Adjust opacity based on brightness - darker content gets brighter glow
        const brightnessMultiplier = 1.2 - currentColors.value.brightness * 0.4;
        return {
            '--glow-color-primary': currentColors.value.primary,
            '--glow-color-secondary': currentColors.value.secondary,
            '--glow-opacity': baseOpacity * brightnessMultiplier,
        };
    });

    async function loadSprite(url: string): Promise<HTMLImageElement | null> {
        if (!import.meta.client) return null;
        if (spriteCache.has(url)) {
            return spriteCache.get(url)!;
        }

        return new Promise((resolve) => {
            const img = new Image();
            img.crossOrigin = 'anonymous';
            img.onload = () => {
                spriteCache.set(url, img);
                resolve(img);
            };
            img.onerror = () => resolve(null);
            img.src = url;
        });
    }

    async function extractColorsFromThumbnail(): Promise<ExtractedColors> {
        if (!import.meta.client) return DEFAULT_COLORS;
        if (!thumbnailUrl.value) return DEFAULT_COLORS;

        return new Promise((resolve) => {
            const img = new Image();
            img.crossOrigin = 'anonymous';
            img.onload = () => resolve(extractFromImage(img));
            img.onerror = () => resolve(DEFAULT_COLORS);
            img.src = thumbnailUrl.value;
        });
    }

    async function precomputeKeyframes() {
        if (vttCues.value.length === 0 || isProcessing.value) return;

        isProcessing.value = true;
        const newKeyframes: ColorKeyframe[] = [];

        // Group cues by sprite URL to minimize image loads
        const cuesByUrl = new Map<string, VttCue[]>();
        for (const cue of vttCues.value) {
            const existing = cuesByUrl.get(cue.url);
            if (existing) {
                existing.push(cue);
            } else {
                cuesByUrl.set(cue.url, [cue]);
            }
        }

        // Process each sprite sheet
        for (const [url, cues] of cuesByUrl) {
            const img = await loadSprite(url);
            if (!img) continue;

            // Process in batches to avoid blocking UI
            const batchSize = 10;
            for (let i = 0; i < cues.length; i += batchSize) {
                const batch = cues.slice(i, i + batchSize);

                for (const cue of batch) {
                    const colors = extractFromImageRegion(img, cue.x, cue.y, cue.w, cue.h);
                    newKeyframes.push({
                        time: cue.start,
                        colors,
                    });
                }

                // Yield to browser between batches
                if (i + batchSize < cues.length) {
                    await new Promise((resolve) => setTimeout(resolve, 0));
                }
            }
        }

        // Sort keyframes by time
        newKeyframes.sort((a, b) => a.time - b.time);
        keyframes.value = newKeyframes;
        isProcessing.value = false;
    }

    // Binary search for the last keyframe at or before the given time
    function findKeyframeIndex(time: number): number {
        const kfs = keyframes.value;
        let lo = 0;
        let hi = kfs.length - 1;
        let result = -1;

        while (lo <= hi) {
            const mid = (lo + hi) >>> 1;
            if (kfs[mid]!.time <= time) {
                result = mid;
                lo = mid + 1;
            } else {
                hi = mid - 1;
            }
        }
        return result;
    }

    function interpolateColors(time: number): ExtractedColors {
        const kfs = keyframes.value;
        if (kfs.length === 0) return currentColors.value;

        const beforeIdx = findKeyframeIndex(time);

        if (beforeIdx < 0) return kfs[0]!.colors;
        if (beforeIdx >= kfs.length - 1) return kfs[beforeIdx]!.colors;

        const before = kfs[beforeIdx]!;
        const after = kfs[beforeIdx + 1]!;

        const duration = after.time - before.time;
        if (duration <= 0) return before.colors;

        const rawT = (time - before.time) / duration;
        const t = easeInOutQuad(Math.max(0, Math.min(1, rawT)));

        return {
            primary: lerpColor(before.colors.primary, after.colors.primary, t),
            secondary: lerpColor(before.colors.secondary, after.colors.secondary, t),
            brightness:
                before.colors.brightness + (after.colors.brightness - before.colors.brightness) * t,
        };
    }

    function updateLoop() {
        const getCurrentTime = playerRef.value?.getCurrentTime;
        if (getCurrentTime) {
            const currentTime = getCurrentTime();
            const now = performance.now();

            // Throttle to ~20fps for color updates
            if (now - lastUpdateTime > 50) {
                currentColors.value = interpolateColors(currentTime);
                lastUpdateTime = now;
            }
        }

        animationId = requestAnimationFrame(updateLoop);
    }

    function startAnimation() {
        if (animationId === null) {
            animationId = requestAnimationFrame(updateLoop);
        }
    }

    function stopAnimation() {
        if (animationId !== null) {
            cancelAnimationFrame(animationId);
            animationId = null;
        }
    }

    // Start/stop the rAF loop based on playback state to avoid idle CPU usage
    watch(isPlaying, (playing) => {
        if (playing && keyframes.value.length > 0) {
            startAnimation();
        } else {
            stopAnimation();
        }
    });

    async function initialize() {
        // Start with thumbnail colors
        const thumbnailColors = await extractColorsFromThumbnail();
        currentColors.value = thumbnailColors;

        // Precompute sprite keyframes if VTT cues are already available
        if (vttCues.value.length > 0) {
            await precomputeKeyframes();
        }

        // Start animation only if already playing
        if (isPlaying.value && keyframes.value.length > 0) {
            startAnimation();
        }
    }

    // Watch for VTT cues to become available
    watch(
        vttCues,
        async (newCues) => {
            if (newCues.length > 0 && keyframes.value.length === 0) {
                await precomputeKeyframes();
                // Start animation if playing and keyframes just became available
                if (isPlaying.value && keyframes.value.length > 0) {
                    startAnimation();
                }
            }
        },
        { deep: true },
    );

    // Watch for thumbnail URL changes (update thumbnail glow when not playing)
    watch(thumbnailUrl, async () => {
        if (!isPlaying.value || keyframes.value.length === 0) {
            const thumbnailColors = await extractColorsFromThumbnail();
            currentColors.value = thumbnailColors;
        }
    });

    onMounted(() => {
        initialize();
    });

    onBeforeUnmount(() => {
        stopAnimation();
        spriteCache.clear();
    });

    return {
        glowStyle,
        currentColors,
        isProcessing,
    };
}
