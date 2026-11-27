import type { VttCue } from './useVttParser';
import type { ExtractedColors } from './useColorExtractor';

interface ColorKeyframe {
    time: number;
    colors: ExtractedColors;
}

interface GlowStyle {
    [key: `--${string}`]: string | number;
    opacity: number;
}

/** How often the playing position is checked against the keyframes (ms). */
const CHECK_INTERVAL_MS = 500;

const DEFAULT_COLORS: ExtractedColors = {
    primary: 'rgb(0, 0, 0)',
    secondary: 'rgb(0, 0, 0)',
    brightness: 1,
};

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

    // Two glow layers crossfaded with their opacity: a new colour is painted once on the hidden
    // layer, then the fade runs on the compositor. Repainting the blurred gradients 20 times a
    // second (the former interpolation) dropped about a fifth of the video frames as soon as a
    // few tabs played at once.
    const layerColors = ref<[ExtractedColors, ExtractedColors]>([DEFAULT_COLORS, DEFAULT_COLORS]);
    const activeLayer = ref<0 | 1>(0);

    let timer: ReturnType<typeof setInterval> | null = null;
    let shownKeyframe = -1;

    function layerStyle(colors: ExtractedColors, visible: boolean): GlowStyle {
        // Darker content gets a brighter glow
        const brightnessMultiplier = 1.2 - colors.brightness * 0.4;
        return {
            '--glow-color-primary': colors.primary,
            '--glow-color-secondary': colors.secondary,
            '--glow-opacity': brightnessMultiplier,
            opacity: visible ? 1 : 0,
        };
    }

    /** Styles of the two glow layers (each holds a primary and a secondary glow). */
    const glowLayers = computed<[GlowStyle, GlowStyle]>(() => [
        layerStyle(layerColors.value[0], activeLayer.value === 0),
        layerStyle(layerColors.value[1], activeLayer.value === 1),
    ]);

    function sameColors(a: ExtractedColors, b: ExtractedColors): boolean {
        return a.primary === b.primary && a.secondary === b.secondary;
    }

    /** Shows colours: painted on the hidden layer, which then fades in. */
    function showColors(colors: ExtractedColors) {
        currentColors.value = colors;
        if (sameColors(layerColors.value[activeLayer.value], colors)) return;
        const next = activeLayer.value === 0 ? 1 : 0;
        const pair: [ExtractedColors, ExtractedColors] = [...layerColors.value];
        pair[next] = colors;
        layerColors.value = pair;
        activeLayer.value = next;
    }

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

    /** Shows the colours of the keyframe at the playing position, when it changed. */
    function check() {
        const getCurrentTime = playerRef.value?.getCurrentTime;
        if (!getCurrentTime || keyframes.value.length === 0) return;
        const idx = Math.max(0, findKeyframeIndex(getCurrentTime()));
        if (idx === shownKeyframe) return;
        shownKeyframe = idx;
        const kf = keyframes.value[idx];
        if (kf) showColors(kf.colors);
    }

    function startAnimation() {
        if (timer === null) {
            check();
            timer = setInterval(check, CHECK_INTERVAL_MS);
        }
    }

    function stopAnimation() {
        if (timer !== null) {
            clearInterval(timer);
            timer = null;
        }
    }

    // Start/stop the position checks based on playback state to avoid idle CPU usage
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
        showColors(thumbnailColors);

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
            shownKeyframe = -1;
            showColors(thumbnailColors);
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
        glowLayers,
        currentColors,
        isProcessing,
    };
}
