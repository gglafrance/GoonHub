import type { ShallowRef } from 'vue';
import type videojs from 'video.js';
import type { Scene } from '~/types/scene';

type Player = ReturnType<typeof videojs>;

interface VideoJsPlayerWithControls extends Player {
    controlBar?: {
        progressControl?: {
            seekBar?: {
                el: () => HTMLElement | undefined;
            };
        };
    };
}

interface ReplayHeatmapOptions {
    player: ShallowRef<Player | null>;
    scene: Ref<Scene | undefined>;
    flushIntervalMs?: number;
}

const SVG_NS = 'http://www.w3.org/2000/svg';
let gradientCounter = 0;

/**
 * "Most replayed" heatmap of the player: records the parts of the scene that are actually
 * played (seeking does not count the skipped part) and draws the current user's histogram
 * as a curve above the progress bar. Watched ranges are sent in batches: every
 * flushIntervalMs while playing, and on pause, end, scene change and when leaving.
 * Both halves follow the user's Settings > Player toggles.
 */
export const useReplayHeatmap = (options: ReplayHeatmapOptions) => {
    const { player, scene, flushIntervalMs = 30000 } = options;

    const settingsStore = useSettingsStore();
    const { getReplayHeatmap, recordReplayHeatmap } = useApiScenes();

    const tracker = createWatchedRangeTracker();
    const buckets = ref<number[]>([]);
    const sceneId = computed(() => scene.value?.id ?? null);

    let lastFlush = Date.now();
    let knownDuration = 0;
    let loadToken = 0;
    let container: HTMLDivElement | null = null;
    let areaPath: SVGPathElement | null = null;
    let linePath: SVGPathElement | null = null;

    // --- Recording ---

    const flush = async (keepalive = false, id = sceneId.value) => {
        lastFlush = Date.now();
        const ranges = tracker.take();
        if (!id || ranges.length === 0 || !settingsStore.recordReplayHeatmap) return;
        try {
            const result = await recordReplayHeatmap(id, ranges, knownDuration, keepalive);
            if (result.recorded && id === sceneId.value && settingsStore.showReplayHeatmap) {
                buckets.value = result.buckets;
            }
        } catch {
            // Silently fail - the heatmap is not critical
        }
    };

    const onTimeUpdate = () => {
        const p = player.value;
        if (!p || !sceneId.value || !settingsStore.recordReplayHeatmap) return;
        const duration = p.duration();
        if (duration && Number.isFinite(duration)) knownDuration = duration;
        if (p.seeking() || p.paused()) {
            tracker.close();
            return;
        }
        tracker.advance(p.currentTime() ?? 0, p.playbackRate() ?? 1);
        if (Date.now() - lastFlush >= flushIntervalMs) flush();
    };

    const onSeeking = () => tracker.close();

    const onStop = () => {
        tracker.close();
        flush();
    };

    const onPageHide = () => flush(true);

    const onVisibilityChange = () => {
        if (document.visibilityState === 'hidden') flush(true);
    };

    // --- Display ---

    const render = () => {
        if (!container || !areaPath || !linePath) return;
        const paths = settingsStore.showReplayHeatmap
            ? buildReplayHeatmapPaths(buckets.value)
            : null;
        if (!paths) {
            container.style.display = 'none';
            return;
        }
        areaPath.setAttribute('d', paths.area);
        linePath.setAttribute('d', paths.line);
        container.style.display = '';
    };

    const load = async () => {
        const token = ++loadToken;
        buckets.value = [];
        const id = sceneId.value;
        if (!id || !settingsStore.showReplayHeatmap) return;
        try {
            const heatmap = await getReplayHeatmap(id);
            if (token === loadToken) buckets.value = heatmap.buckets ?? [];
        } catch {
            // No heatmap
        }
    };

    const createOverlay = (progressHolder: HTMLElement) => {
        const gradientId = `vjs-replay-heatmap-gradient-${++gradientCounter}`;
        container = document.createElement('div');
        container.className = 'vjs-replay-heatmap';
        container.style.display = 'none';

        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 1000 100');
        svg.setAttribute('preserveAspectRatio', 'none');
        svg.setAttribute('aria-hidden', 'true');

        const defs = document.createElementNS(SVG_NS, 'defs');
        const gradient = document.createElementNS(SVG_NS, 'linearGradient');
        gradient.setAttribute('id', gradientId);
        gradient.setAttribute('x1', '0');
        gradient.setAttribute('y1', '0');
        gradient.setAttribute('x2', '0');
        gradient.setAttribute('y2', '1');
        for (const [offset, opacity] of [
            ['0', '0.32'],
            ['1', '0.04'],
        ] as const) {
            const stop = document.createElementNS(SVG_NS, 'stop');
            stop.setAttribute('offset', offset);
            stop.setAttribute('stop-color', '#ffffff');
            stop.setAttribute('stop-opacity', opacity);
            gradient.appendChild(stop);
        }
        defs.appendChild(gradient);
        svg.appendChild(defs);

        areaPath = document.createElementNS(SVG_NS, 'path');
        areaPath.setAttribute('class', 'vjs-replay-heatmap-area');
        areaPath.setAttribute('fill', `url(#${gradientId})`);
        svg.appendChild(areaPath);

        linePath = document.createElementNS(SVG_NS, 'path');
        linePath.setAttribute('class', 'vjs-replay-heatmap-line');
        linePath.setAttribute('fill', 'none');
        linePath.setAttribute('vector-effect', 'non-scaling-stroke');
        svg.appendChild(linePath);

        container.appendChild(svg);
        progressHolder.appendChild(container);
    };

    // --- Lifecycle ---

    // Created with the component (before ScenePlayer's own source watcher) so that a scene
    // change sends what was watched of the previous scene before the player is reset.
    watch(sceneId, (_id, previousId) => {
        tracker.close();
        if (previousId) flush(false, previousId);
        tracker.reset();
        knownDuration = 0;
        load();
    });
    watch(
        () => settingsStore.showReplayHeatmap,
        () => load(),
    );
    watch(
        () => settingsStore.recordReplayHeatmap,
        (record) => {
            if (!record) tracker.reset();
        },
    );
    watch(buckets, render);

    const setup = () => {
        const p = player.value;
        if (!p) return;

        const progressHolder = (
            p as unknown as VideoJsPlayerWithControls
        ).controlBar?.progressControl?.seekBar?.el();
        if (progressHolder) createOverlay(progressHolder);

        p.on('timeupdate', onTimeUpdate);
        p.on('seeking', onSeeking);
        p.on('pause', onStop);
        p.on('ended', onStop);
        window.addEventListener('pagehide', onPageHide);
        document.addEventListener('visibilitychange', onVisibilityChange);

        load();
    };

    const cleanup = () => {
        const p = player.value;
        if (p) {
            p.off('timeupdate', onTimeUpdate);
            p.off('seeking', onSeeking);
            p.off('pause', onStop);
            p.off('ended', onStop);
        }
        window.removeEventListener('pagehide', onPageHide);
        document.removeEventListener('visibilitychange', onVisibilityChange);

        tracker.close();
        flush(true);
        loadToken++;

        if (container?.parentNode) container.parentNode.removeChild(container);
        container = null;
        areaPath = null;
        linePath = null;
    };

    return { buckets, setup, cleanup, flush };
};
