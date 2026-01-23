export interface VttCue {
    start: number;
    end: number;
    url: string;
    x: number;
    y: number;
    w: number;
    h: number;
}

export function parseVttTime(timeStr: string): number {
    const parts = timeStr.trim().split(':');
    if (parts.length < 3) return 0;
    const hours = parseInt(parts[0] || '0');
    const minutes = parseInt(parts[1] || '0');
    const secParts = (parts[2] || '0').split('.');
    const seconds = parseInt(secParts[0] || '0');
    const millis = parseInt(secParts[1] || '0');
    return hours * 3600 + minutes * 60 + seconds + millis / 1000;
}

export function useVttParser() {
    const vttCues = ref<VttCue[]>([]);

    async function loadVttCues(url: string) {
        try {
            const response = await fetch(url);
            const text = await response.text();
            const cues: VttCue[] = [];

            const blocks = text.split('\n\n');
            for (const block of blocks) {
                const lines = block.trim().split('\n');
                for (let i = 0; i < lines.length; i++) {
                    const line = lines[i];
                    if (line && line.includes('-->')) {
                        const [startStr, endStr] = line.split('-->');
                        if (!startStr || !endStr) continue;
                        const start = parseVttTime(startStr);
                        const end = parseVttTime(endStr);
                        const urlLine = lines[i + 1]?.trim();
                        if (!urlLine) continue;

                        const hashIndex = urlLine.indexOf('#xywh=');
                        if (hashIndex === -1) continue;

                        const spriteUrl = urlLine.substring(0, hashIndex);
                        const coords = urlLine
                            .substring(hashIndex + 6)
                            .split(',')
                            .map(Number);
                        cues.push({
                            start,
                            end,
                            url: spriteUrl,
                            x: coords[0] ?? 0,
                            y: coords[1] ?? 0,
                            w: coords[2] ?? 0,
                            h: coords[3] ?? 0,
                        });
                    }
                }
            }
            vttCues.value = cues;
        } catch (e: unknown) {
            console.error('Failed to load VTT cues:', e);
        }
    }

    return {
        vttCues,
        loadVttCues,
    };
}
