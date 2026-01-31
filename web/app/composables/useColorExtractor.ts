export interface ExtractedColors {
    primary: string;
    secondary: string;
    brightness: number;
}

const DEFAULT_COLORS: ExtractedColors = {
    primary: 'rgb(0, 0, 0)',
    secondary: 'rgb(0, 0, 0)',
    brightness: 1,
};

interface ColorData {
    r: number;
    g: number;
    b: number;
    weight: number;
}

function rgbToString(r: number, g: number, b: number): string {
    return `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`;
}

function getSaturation(r: number, g: number, b: number): number {
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    if (max === 0) return 0;
    return (max - min) / max;
}

function getBrightness(r: number, g: number, b: number): number {
    return (r * 0.299 + g * 0.587 + b * 0.114) / 255;
}

function colorDistance(c1: ColorData, c2: ColorData): number {
    return Math.sqrt(
        Math.pow(c1.r - c2.r, 2) + Math.pow(c1.g - c2.g, 2) + Math.pow(c1.b - c2.b, 2),
    );
}

function quantize(value: number, levels: number = 16): number {
    const step = 256 / levels;
    return Math.floor(value / step) * step + step / 2;
}

export function useColorExtractor() {
    // SSR guard - only create canvas on client
    const canvas = import.meta.client ? document.createElement('canvas') : null;
    const ctx = canvas?.getContext('2d', { willReadFrequently: true }) ?? null;

    function extractFromImageRegion(
        img: HTMLImageElement,
        sx: number,
        sy: number,
        sw: number,
        sh: number,
    ): ExtractedColors {
        if (!ctx) return DEFAULT_COLORS;

        if (!canvas) return DEFAULT_COLORS;

        const sampleSize = 8;
        canvas.width = sampleSize;
        canvas.height = sampleSize;

        try {
            ctx.drawImage(img, sx, sy, sw, sh, 0, 0, sampleSize, sampleSize);
            const imageData = ctx.getImageData(0, 0, sampleSize, sampleSize);
            const data = imageData.data;

            const colorMap = new Map<string, ColorData>();
            let totalBrightness = 0;
            let pixelCount = 0;

            for (let i = 0; i < data.length; i += 4) {
                const r = data[i];
                const g = data[i + 1];
                const b = data[i + 2];

                if (r === undefined || g === undefined || b === undefined) continue;

                const brightness = getBrightness(r, g, b);
                totalBrightness += brightness;
                pixelCount++;

                // Skip near-black and near-white pixels
                if (brightness < 0.1 || brightness > 0.9) continue;

                const saturation = getSaturation(r, g, b);
                // Skip very desaturated pixels
                if (saturation < 0.15) continue;

                // Quantize colors to reduce unique values
                const qr = quantize(r);
                const qg = quantize(g);
                const qb = quantize(b);
                const key = `${qr},${qg},${qb}`;

                // Weight by saturation - vibrant colors are more impactful
                const weight = 1 + saturation * 2;

                const existing = colorMap.get(key);
                if (existing) {
                    existing.weight += weight;
                    // Accumulate actual colors for averaging
                    existing.r += r * weight;
                    existing.g += g * weight;
                    existing.b += b * weight;
                } else {
                    colorMap.set(key, {
                        r: r * weight,
                        g: g * weight,
                        b: b * weight,
                        weight,
                    });
                }
            }

            const avgBrightness = pixelCount > 0 ? totalBrightness / pixelCount : 0.5;

            if (colorMap.size === 0) {
                return { ...DEFAULT_COLORS, brightness: avgBrightness };
            }

            // Sort by weighted frequency
            const sortedColors = Array.from(colorMap.values())
                .map((c) => ({
                    r: c.r / c.weight,
                    g: c.g / c.weight,
                    b: c.b / c.weight,
                    weight: c.weight,
                }))
                .sort((a, b) => b.weight - a.weight);

            const primary = sortedColors[0];
            if (!primary) {
                return { ...DEFAULT_COLORS, brightness: avgBrightness };
            }

            // Find secondary color that's distinct from primary
            let secondary = sortedColors[1];
            for (let i = 1; i < sortedColors.length; i++) {
                const candidate = sortedColors[i];
                if (candidate && colorDistance(primary, candidate) > 50) {
                    secondary = candidate;
                    break;
                }
            }

            return {
                primary: rgbToString(primary.r, primary.g, primary.b),
                secondary: secondary
                    ? rgbToString(secondary.r, secondary.g, secondary.b)
                    : rgbToString(primary.r * 0.8, primary.g * 0.8, primary.b * 0.8),
                brightness: avgBrightness,
            };
        } catch {
            // CORS or other error
            return DEFAULT_COLORS;
        }
    }

    function extractFromImage(img: HTMLImageElement): ExtractedColors {
        return extractFromImageRegion(img, 0, 0, img.naturalWidth, img.naturalHeight);
    }

    return {
        extractFromImage,
        extractFromImageRegion,
        DEFAULT_COLORS,
    };
}
