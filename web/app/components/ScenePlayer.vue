<script setup lang="ts">
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Scene, ScenePlayback } from '~/types/scene';
import type { Marker } from '~/types/marker';
import type { LocationQuery } from 'vue-router';

type Player = ReturnType<typeof videojs>;

// Theater mode button interface for type safety
interface TheaterModeButtonInstance {
    setTheaterMode: (active: boolean) => void;
    _onToggle: (() => void) | null;
}

// Scrubber toggle button interface for type safety
interface ScrubberToggleButtonInstance {
    setActive: (active: boolean) => void;
    _onToggle: (() => void) | null;
}

// Playlist button interface for type safety
interface PlaylistButtonInstance {
    setDisabled: (disabled: boolean) => void;
    _onClick: (() => void) | null;
}

// AB loop button interfaces for type safety
interface ABLoopTimeButtonInstance {
    updateTime: (formatted: string) => void;
    setActive: (active: boolean) => void;
    _onLeftClick: (() => void) | null;
    _onRightClick: (() => void) | null;
}

interface ABLoopToggleButtonInstance {
    setEnabled: (enabled: boolean) => void;
    _onToggle: (() => void) | null;
}

// Register theater mode button component
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const ButtonClass = videojs.getComponent('Button') as any;

if (ButtonClass) {
    class TheaterModeButton extends ButtonClass {
        private _theaterMode: boolean = false;
        public _onToggle: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._theaterMode = options?.theaterMode ?? false;
            this._onToggle = options?.onToggle ?? null;
            this.controlText(this._theaterMode ? 'Exit Theater Mode' : 'Theater Mode');
            this.updateIcon();
        }

        buildCSSClass() {
            return `vjs-theater-mode-control ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onToggle) {
                this._onToggle();
            }
        }

        setTheaterMode(active: boolean) {
            this._theaterMode = active;
            this.controlText(active ? 'Exit Theater Mode' : 'Theater Mode');
            this.updateIcon();
        }

        updateIcon() {
            const el = this.el();
            if (el) {
                el.innerHTML = this._theaterMode
                    ? `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="vjs-theater-icon"><path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3"/></svg>`
                    : `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="vjs-theater-icon"><path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/></svg>`;
            }
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('TheaterModeButton', TheaterModeButton as any);

    // Shows or hides the thumbnail scrubber strip under the player (watch page)
    class ScrubberToggleButton extends ButtonClass {
        public _onToggle: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onToggle = options?.onToggle ?? null;
            this.setActive(options?.active ?? false);
            const el = this.el();
            if (el) {
                el.insertAdjacentHTML(
                    'beforeend',
                    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="vjs-scrubber-icon" aria-hidden="true"><rect x="2" y="7" width="20" height="10" rx="1.5"/><path d="M8.5 7v10M15.5 7v10M12 3v3M12 18v3"/></svg>`,
                );
            }
        }

        buildCSSClass() {
            return `vjs-scrubber-toggle-control ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onToggle) this._onToggle();
        }

        setActive(active: boolean) {
            this.controlText(active ? 'Hide Thumbnail Scrubber' : 'Show Thumbnail Scrubber');
            const el = this.el();
            if (el) el.classList.toggle('vjs-scrubber-toggle-active', active);
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('ScrubberToggleButton', ScrubberToggleButton as any);

    // Playlist previous button
    class PlaylistPrevButton extends ButtonClass {
        public _onClick: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onClick = options?.onClick ?? null;
            this.controlText('Previous');
            this.setIcon();
        }

        buildCSSClass() {
            return `vjs-playlist-prev-control ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onClick) this._onClick();
        }

        setDisabled(disabled: boolean) {
            const el = this.el();
            if (el) {
                if (disabled) {
                    el.classList.add('vjs-playlist-btn-disabled');
                } else {
                    el.classList.remove('vjs-playlist-btn-disabled');
                }
            }
        }

        setIcon() {
            const el = this.el();
            if (el) {
                el.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="vjs-playlist-icon"><path d="M9.195 18.44c1.25.714 2.805-.189 2.805-1.629v-2.34l6.945 3.968c1.25.715 2.805-.188 2.805-1.628V7.19c0-1.44-1.555-2.343-2.805-1.628L12 9.53v-2.34c0-1.44-1.555-2.343-2.805-1.628l-7.108 4.061c-1.26.72-1.26 2.536 0 3.256l7.108 4.061Z"/></svg>`;
            }
        }
    }

    // Playlist next button
    class PlaylistNextButton extends ButtonClass {
        public _onClick: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onClick = options?.onClick ?? null;
            this.controlText('Next');
            this.setIcon();
        }

        buildCSSClass() {
            return `vjs-playlist-next-control ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onClick) this._onClick();
        }

        setDisabled(disabled: boolean) {
            const el = this.el();
            if (el) {
                if (disabled) {
                    el.classList.add('vjs-playlist-btn-disabled');
                } else {
                    el.classList.remove('vjs-playlist-btn-disabled');
                }
            }
        }

        setIcon() {
            const el = this.el();
            if (el) {
                el.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="vjs-playlist-icon"><path d="M5.055 7.06C3.805 6.347 2.25 7.25 2.25 8.689v6.622c0 1.44 1.555 2.343 2.805 1.628L12 13.471v2.34c0 1.44 1.555 2.343 2.805 1.628l7.108-4.061c1.26-.72 1.26-2.536 0-3.256l-7.108-4.061C13.555 5.346 12 6.249 12 7.689v2.34L5.055 7.06Z"/></svg>`;
            }
        }
    }

    // AB Loop start time button: left-click sets start, right-click seeks to start
    class ABLoopStartButton extends ButtonClass {
        public _onLeftClick: (() => void) | null = null;
        public _onRightClick: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onLeftClick = options?.onLeftClick ?? null;
            this._onRightClick = options?.onRightClick ?? null;
            this.controlText('Loop Start: click to set, right-click to seek');
            this.render();
            // Prevent context menu on right-click
            this.el().addEventListener('contextmenu', (e: Event) => {
                e.preventDefault();
                if (this._onRightClick) this._onRightClick();
            });
        }

        buildCSSClass() {
            return `vjs-ab-loop-time vjs-ab-loop-start ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onLeftClick) this._onLeftClick();
        }

        render() {
            const el = this.el();
            if (el)
                el.innerHTML = `<span class="vjs-ab-loop-label">A</span><span class="vjs-ab-loop-value">--:--</span>`;
        }

        updateTime(formatted: string) {
            const el = this.el();
            if (!el) return;
            const valueEl = el.querySelector('.vjs-ab-loop-value');
            if (valueEl) valueEl.textContent = formatted;
        }

        setActive(active: boolean) {
            const el = this.el();
            if (el) el.classList.toggle('vjs-ab-loop-time-active', active);
        }
    }

    // AB Loop end time button: left-click sets end, right-click seeks to end
    class ABLoopEndButton extends ButtonClass {
        public _onLeftClick: (() => void) | null = null;
        public _onRightClick: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onLeftClick = options?.onLeftClick ?? null;
            this._onRightClick = options?.onRightClick ?? null;
            this.controlText('Loop End: click to set, right-click to seek');
            this.render();
            this.el().addEventListener('contextmenu', (e: Event) => {
                e.preventDefault();
                if (this._onRightClick) this._onRightClick();
            });
        }

        buildCSSClass() {
            return `vjs-ab-loop-time vjs-ab-loop-end ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onLeftClick) this._onLeftClick();
        }

        render() {
            const el = this.el();
            if (el)
                el.innerHTML = `<span class="vjs-ab-loop-label">B</span><span class="vjs-ab-loop-value">--:--</span>`;
        }

        updateTime(formatted: string) {
            const el = this.el();
            if (!el) return;
            const valueEl = el.querySelector('.vjs-ab-loop-value');
            if (valueEl) valueEl.textContent = formatted;
        }

        setActive(active: boolean) {
            const el = this.el();
            if (el) el.classList.toggle('vjs-ab-loop-time-active', active);
        }
    }

    // AB Loop toggle button: enables/disables looping
    class ABLoopToggleButton extends ButtonClass {
        private _enabled: boolean = false;
        public _onToggle: (() => void) | null = null;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        constructor(player: Player, options?: any) {
            super(player, options);
            this._onToggle = options?.onToggle ?? null;
            this.controlText('Toggle A/B Loop');
            this.updateIcon();
        }

        buildCSSClass() {
            return `vjs-ab-loop-toggle ${super.buildCSSClass()}`;
        }

        handleClick() {
            if (this._onToggle) this._onToggle();
        }

        setEnabled(enabled: boolean) {
            this._enabled = enabled;
            const el = this.el();
            if (el) el.classList.toggle('vjs-ab-loop-enabled', enabled);
            this.updateIcon();
        }

        updateIcon() {
            const el = this.el();
            if (!el) return;
            // Repeat/loop icon
            el.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="vjs-ab-loop-icon"><path d="M17 2l4 4-4 4"/><path d="M3 11v-1a4 4 0 0 1 4-4h14"/><path d="M7 22l-4-4 4-4"/><path d="M21 13v1a4 4 0 0 1-4 4H3"/></svg>`;
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('PlaylistPrevButton', PlaylistPrevButton as any);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('PlaylistNextButton', PlaylistNextButton as any);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('ABLoopStartButton', ABLoopStartButton as any);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('ABLoopEndButton', ABLoopEndButton as any);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    videojs.registerComponent('ABLoopToggleButton', ABLoopToggleButton as any);
}

const props = defineProps<{
    sceneUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    loop?: boolean;
    defaultVolume?: number;
    scene?: Scene;
    startTime?: number;
    // Marker range to loop once the player is ready (e.g. opened from a marker link)
    loopRange?: { start: number; end: number } | null;
    markers?: Marker[];
    playlistMode?: boolean;
    hasNext?: boolean;
    hasPrevious?: boolean;
    // Whether the browser can play the original (defaults to scene.playback)
    playback?: ScenePlayback | null;
    // On-the-fly conversion endpoint, used when the browser can't play the original
    transcodeUrl?: string;
    // Scene duration in seconds when no scene is passed (shown for converted streams)
    durationHint?: number;
    // Control bar button that shows/hides the thumbnail scrubber (saved in the user settings)
    scrubberToggle?: boolean;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    ended: [];
    error: [error: unknown];
    viewRecorded: [];
    next: [];
    previous: [];
}>();

const videoElement = ref<HTMLVideoElement>();
const player = shallowRef<Player | null>(null);
const settingsStore = useSettingsStore();
const theaterModeButton = shallowRef<TheaterModeButtonInstance | null>(null);
const scrubberToggleBtn = shallowRef<ScrubberToggleButtonInstance | null>(null);
const playlistPrevBtn = shallowRef<PlaylistButtonInstance | null>(null);
const playlistNextBtn = shallowRef<PlaylistButtonInstance | null>(null);
const abLoopStartBtn = shallowRef<ABLoopTimeButtonInstance | null>(null);
const abLoopEndBtn = shallowRef<ABLoopTimeButtonInstance | null>(null);
const abLoopToggleBtn = shallowRef<ABLoopToggleButtonInstance | null>(null);
const abLoop = useABLoop(player);
const { vttCues, loadVttCues } = useVttParser();
const { setup: setupThumbnailPreview, cleanup: cleanupThumbnailPreview } = useThumbnailPreview(
    player,
    vttCues,
);
const {
    setup: setupMarkerIndicators,
    cleanup: cleanupMarkerIndicators,
    update: updateMarkerIndicators,
} = useMarkerIndicators(
    player,
    computed(() => props.markers ?? []),
    (marker) => playMarker(marker),
);

// A/B loop range last applied from a marker, so that clicking a point marker afterwards
// only clears a loop the user did not set by hand.
let markerLoopRange: { start: number; end: number } | null = null;

// A range marker sets the A/B loop points to its start and end; the "Loop Marker Ranges"
// setting decides whether looping starts right away (an active loop stays active). A point
// marker clears a loop previously set from a marker.
const applyMarkerLoop = (marker: Pick<Marker, 'timestamp' | 'end_timestamp'>) => {
    if (markerHasRange(marker)) {
        // Keep looping when the loop is already on (e.g. turned on with the A/B toggle)
        abLoop.setRange(
            marker.timestamp,
            marker.end_timestamp,
            settingsStore.markerLoop || abLoop.enabled.value,
        );
        markerLoopRange = { start: marker.timestamp, end: marker.end_timestamp };
        return;
    }
    if (
        markerLoopRange &&
        abLoop.pointA.value === markerLoopRange.start &&
        abLoop.pointB.value === markerLoopRange.end
    ) {
        abLoop.clear();
    }
    markerLoopRange = null;
};

// Seeks to a marker and plays it, looping its range when applicable.
const playMarker = (marker: Pick<Marker, 'timestamp' | 'end_timestamp'>) => {
    const p = player.value;
    if (!p) return;
    applyMarkerLoop(marker);
    p.currentTime(marker.timestamp);
    p.play()?.catch(() => {});
};

// Touch gestures (double-tap seek, swipes, long-press 2x), feedback teleported into the player
const playerEl = shallowRef<HTMLElement | null>(null);
const {
    indicator: gestureIndicator,
    setup: setupGestures,
    cleanup: cleanupGestures,
} = usePlayerGestures({
    player,
    vttCues,
    enabled: computed(() => settingsStore.playerGestures),
    seekStep: computed(() => settingsStore.gestureSeekStep),
});

// Picture-in-picture: the button only shows where the browser supports it, and a scene playing
// in picture-in-picture keeps playing when its page is left
const pipSupported = import.meta.client && !!document.pictureInPictureEnabled;
const pipKeeper = usePipKeeper();
const route = useRoute();
// Page of the current scene (updated when the scene changes, not when the page is left)
const snapshotRoute = () => ({ path: route.path, query: { ...route.query } });
let pageRoute = snapshotRoute();

// From the picture-in-picture window back to the tab: reopen this page at the current time
const returnToScene = (time: number) => {
    const query: LocationQuery = { ...pageRoute.query, t: String(Math.floor(time)) };
    delete query.end;
    navigateTo({ path: pageRoute.path, query });
};
const transcode = useTranscodePlayback(player, {
    playback: () => props.playback ?? props.scene?.playback,
    transcodeUrl: () => props.transcodeUrl,
    duration: () => props.durationHint ?? props.scene?.duration ?? 0,
    loop: () => props.loop ?? false,
});
const { notice: playbackNotice, active: transcodeActive, mode: transcodeMode } = transcode;
// The original file, as an attachment (offered when the browser can't play it)
const downloadUrl = computed(
    () => `${props.sceneUrl}${props.sceneUrl.includes('?') ? '&' : '?'}download=1`,
);

const sceneRef = computed(() => props.scene);
const { hasRecordedView, setupTracking, cleanup } = useWatchTracking({
    player,
    scene: sceneRef,
});
const { setup: setupReplayHeatmap, cleanup: cleanupReplayHeatmap } = useReplayHeatmap({
    player,
    scene: sceneRef,
});

// Attempt autoplay, falling back to muted if browser blocks it.
// Waits for canplay in Firefox which rejects play() on unbuffered media.
const tryAutoplay = (p: Player) => {
    const attempt = () => {
        p.play()?.catch(() => {
            p.muted(true);
            p.play();
        });
    };
    if (p.readyState() >= 3) {
        attempt();
    } else {
        p.one('canplay', attempt);
    }
};

const aspectRatio = computed(() => {
    if (props.scene?.width && props.scene?.height) {
        return `${props.scene.width} / ${props.scene.height}`;
    }
    return '16 / 9';
});

const isPortrait = computed(() => {
    return props.scene?.width && props.scene?.height && props.scene.height > props.scene.width;
});

const vttUrl = computed(() => {
    if (!props.scene?.vtt_path) return null;
    const base = `/vtt/${props.scene.id}`;
    const v = props.scene.updated_at ? new Date(props.scene.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

onMounted(async () => {
    if (!videoElement.value) return;

    // A scene left in picture-in-picture stops here; the same scene continues where it was
    const pipHandoff = pipKeeper.takeOver();

    player.value = videojs(videoElement.value, {
        controls: true,
        autoplay: props.autoplay ? 'any' : false,
        loop: props.loop ?? false,
        preload: 'auto',
        fill: true,
        playbackRates: [0.5, 0.75, 1, 1.25, 1.5, 2],
        controlBar: {
            children: [
                'playToggle',
                'volumePanel',
                'currentTimeDisplay',
                'timeDivider',
                'durationDisplay',
                'progressControl',
                'remainingTimeDisplay',
                'playbackRateMenuButton',
                ...(settingsStore.abLoopControls
                    ? ['ABLoopStartButton', 'ABLoopEndButton', 'ABLoopToggleButton']
                    : []),
                ...(pipSupported ? ['pictureInPictureToggle'] : []),
                ...(props.scrubberToggle ? ['ScrubberToggleButton'] : []),
                'TheaterModeButton',
                'fullscreenToggle',
            ],
        },
    });

    playerEl.value = player.value.el() as HTMLElement;

    // Get reference to theater mode button and configure it
    const controlBar = player.value.getChild('controlBar');
    if (controlBar) {
        const btn = controlBar.getChild('TheaterModeButton') as unknown as
            | TheaterModeButtonInstance
            | undefined;
        if (btn) {
            theaterModeButton.value = btn;
            btn.setTheaterMode(settingsStore.theaterMode);
            btn._onToggle = () => {
                settingsStore.toggleTheaterMode();
            };
        }

        const scrubberBtn = controlBar.getChild('ScrubberToggleButton') as unknown as
            | ScrubberToggleButtonInstance
            | undefined;
        if (scrubberBtn) {
            scrubberToggleBtn.value = scrubberBtn;
            scrubberBtn.setActive(settingsStore.showScrubber);
            scrubberBtn._onToggle = () => {
                settingsStore.setShowScrubber(!settingsStore.showScrubber);
            };
        }

        // Wire up AB loop buttons
        const startBtn = controlBar.getChild('ABLoopStartButton') as unknown as
            | ABLoopTimeButtonInstance
            | undefined;
        if (startBtn) {
            abLoopStartBtn.value = startBtn;
            startBtn._onLeftClick = () => abLoop.setStart();
            startBtn._onRightClick = () => abLoop.seekToStart();
        }

        const endBtn = controlBar.getChild('ABLoopEndButton') as unknown as
            | ABLoopTimeButtonInstance
            | undefined;
        if (endBtn) {
            abLoopEndBtn.value = endBtn;
            endBtn._onLeftClick = () => abLoop.setEnd();
            endBtn._onRightClick = () => abLoop.seekToEnd();
        }

        const toggleBtn = controlBar.getChild('ABLoopToggleButton') as unknown as
            | ABLoopToggleButtonInstance
            | undefined;
        if (toggleBtn) {
            abLoopToggleBtn.value = toggleBtn;
            toggleBtn._onToggle = () => abLoop.toggleEnabled();
        }

        // Add playlist prev/next buttons when in playlist mode
        if (props.playlistMode) {
            const barEl = controlBar.el() as HTMLElement | null;

            const rawPrev = controlBar.addChild('PlaylistPrevButton', {
                onClick: () => emit('previous'),
            });
            // Move to index 1 (after playToggle)
            if (barEl) barEl.insertBefore(rawPrev.el(), barEl.children[1] ?? null);
            const prevBtn = rawPrev as unknown as PlaylistButtonInstance;
            prevBtn.setDisabled(!props.hasPrevious);
            playlistPrevBtn.value = prevBtn;

            const rawNext = controlBar.addChild('PlaylistNextButton', {
                onClick: () => emit('next'),
            });
            // Move to index 2 (after prev button)
            if (barEl) barEl.insertBefore(rawNext.el(), barEl.children[2] ?? null);
            const nextBtn = rawNext as unknown as PlaylistButtonInstance;
            nextBtn.setDisabled(!props.hasNext);
            playlistNextBtn.value = nextBtn;
        }
    }

    // Set initial volume (video.js uses 0-1 range)
    const volume = props.defaultVolume != null ? props.defaultVolume / 100 : 1;
    player.value.volume(volume);

    player.value.on('play', () => emit('play'));
    player.value.on('pause', () => emit('pause'));
    player.value.on('ended', () => emit('ended'));
    player.value.on('error', (e: unknown) => {
        // Formats the browser can't play are converted on the fly or explained in an overlay
        if (transcode.handleError()) return;
        emit('error', e);
    });

    transcode.attach();
    transcode.init(props.startTime ?? 0, !!props.autoplay);

    // Set up watch tracking (handles timeupdate, ended, and beforeunload)
    setupTracking();

    // Emit viewRecorded when first recorded
    watch(hasRecordedView, (recorded) => {
        if (recorded) emit('viewRecorded');
    });

    // Create AB loop overlay element for the progress bar
    const abLoopOverlay = document.createElement('div');
    abLoopOverlay.className = 'vjs-ab-loop-overlay';
    abLoopOverlay.style.display = 'none';

    const abLoopMarkerA = document.createElement('div');
    abLoopMarkerA.className = 'vjs-ab-loop-marker vjs-ab-loop-marker-a';
    abLoopMarkerA.style.display = 'none';

    player.value.ready(() => {
        setupThumbnailPreview();
        setupMarkerIndicators();
        setupGestures();
        setupReplayHeatmap();
        if (vttUrl.value) {
            loadVttCues(vttUrl.value);
        }

        // Inject AB loop overlay into progress bar
        const progressHolder = player.value!.el().querySelector('.vjs-progress-holder');
        if (progressHolder) {
            progressHolder.appendChild(abLoopOverlay);
            progressHolder.appendChild(abLoopMarkerA);
        }

        // Seek to start time if provided (a converted stream already starts there)
        if (props.startTime && props.startTime > 0 && !transcode.active.value) {
            player.value!.currentTime(props.startTime);
        } else if (pipHandoff && pipHandoff.sceneUrl === props.sceneUrl) {
            player.value!.currentTime(pipHandoff.time);
            if (pipHandoff.playing && !props.autoplay) tryAutoplay(player.value!);
        }

        if (props.loopRange) {
            applyMarkerLoop({
                timestamp: props.loopRange.start,
                end_timestamp: props.loopRange.end,
            });
        }

        if (props.autoplay) {
            tryAutoplay(player.value!);
        }
    });

    // Watch AB loop state to update progress bar overlay
    watch(
        [() => abLoop.progressA.value, () => abLoop.progressB.value, abLoop.enabled],
        ([pA, pB, isEnabled]) => {
            // Show single marker for point A when only A is set
            if (pA !== null && pB === null) {
                abLoopMarkerA.style.display = 'block';
                abLoopMarkerA.style.left = `${pA}%`;
                abLoopOverlay.style.display = 'none';
            }
            // Show range overlay when both points are set
            else if (pA !== null && pB !== null) {
                abLoopMarkerA.style.display = 'none';
                abLoopOverlay.style.display = 'block';
                abLoopOverlay.style.left = `${pA}%`;
                abLoopOverlay.style.width = `${pB - pA}%`;
                // Dim overlay when loop is disabled
                abLoopOverlay.style.opacity = isEnabled ? '1' : '0.4';
            } else {
                abLoopOverlay.style.display = 'none';
                abLoopMarkerA.style.display = 'none';
            }
        },
        { immediate: true },
    );
});

watch(
    () => props.sceneUrl,
    (newUrl) => {
        if (!player.value) return;
        const p = player.value;

        pageRoute = snapshotRoute();

        // Clear AB loop and reset before loading new source
        abLoop.clear();
        markerLoopRange = null;
        p.pause();
        // (a converted stream would reload at 0 first)
        if (!transcode.active.value) p.currentTime(0);
        const playable = transcode.load(newUrl, 0, !!props.autoplay);

        if (props.autoplay && playable) {
            p.ready(() => tryAutoplay(p));
        }
    },
);

watch(vttUrl, (newVttUrl, oldVttUrl) => {
    // The thumbnails of another scene must not show meanwhile (scrubber, gestures)
    if (newVttUrl?.split('?')[0] !== oldVttUrl?.split('?')[0]) vttCues.value = [];
    if (newVttUrl) {
        loadVttCues(newVttUrl);
    }
});

// Apply a marker loop range passed by the page (defined after the sceneUrl watcher so that a
// scene change clears the previous loop first)
watch(
    () => props.loopRange,
    (range) => {
        if (!player.value) return;
        applyMarkerLoop(
            range
                ? { timestamp: range.start, end_timestamp: range.end }
                : { timestamp: 0, end_timestamp: null },
        );
    },
);

// Watch for marker changes
watch(
    () => props.markers,
    () => {
        updateMarkerIndicators();
    },
    { deep: true },
);

// Watch for startTime changes (e.g., when user clicks Resume)
watch(
    () => props.startTime,
    (newStartTime) => {
        if (player.value && newStartTime && newStartTime > 0) {
            player.value.currentTime(newStartTime);
            tryAutoplay(player.value);
        }
    },
);

// Sync theater mode button state when store changes
watch(
    () => settingsStore.theaterMode,
    (isTheaterMode) => {
        if (theaterModeButton.value) {
            theaterModeButton.value.setTheaterMode(isTheaterMode);
        }
    },
);

watch(
    () => settingsStore.showScrubber,
    (shown) => scrubberToggleBtn.value?.setActive(shown),
);

// Sync AB loop button states
watch([abLoop.formattedA, abLoop.pointA], ([formatted]) => {
    if (abLoopStartBtn.value) {
        abLoopStartBtn.value.updateTime(formatted);
        abLoopStartBtn.value.setActive(abLoop.pointA.value !== null);
    }
});

watch([abLoop.formattedB, abLoop.pointB], ([formatted]) => {
    if (abLoopEndBtn.value) {
        abLoopEndBtn.value.updateTime(formatted);
        abLoopEndBtn.value.setActive(abLoop.pointB.value !== null);
    }
});

watch(abLoop.enabled, (isEnabled) => {
    if (abLoopToggleBtn.value) {
        abLoopToggleBtn.value.setEnabled(isEnabled);
    }
});

// Sync playlist button disabled states
watch(
    () => props.hasPrevious,
    (val) => {
        if (playlistPrevBtn.value) playlistPrevBtn.value.setDisabled(!val);
    },
);

watch(
    () => props.hasNext,
    (val) => {
        if (playlistNextBtn.value) playlistNextBtn.value.setDisabled(!val);
    },
);

defineExpose({
    getCurrentTime: () => player.value?.currentTime() ?? 0,
    player,
    vttCues,
    abLoop,
    playMarker,
    // video.js root element: overlays teleported into it also show in fullscreen
    playerEl,
});

onBeforeUnmount(() => {
    transcode.detach();
    cleanup();
    cleanupThumbnailPreview();
    cleanupMarkerIndicators();
    cleanupGestures();
    cleanupReplayHeatmap();
    if (player.value && !pipKeeper.keep(player.value, props.sceneUrl, returnToScene)) {
        player.value.dispose();
    }
});
</script>

<template>
    <div
        class="video-wrapper"
        :class="{ 'video-wrapper--portrait': isPortrait }"
        :style="{ aspectRatio }"
    >
        <video
            ref="videoElement"
            class="video-js vjs-big-play-centered"
            controls
            :poster="posterUrl"
            crossorigin="anonymous"
        >
            <source v-if="transcode.startsDirect" :src="sceneUrl" type="video/mp4" />
        </video>
        <Teleport v-if="playerEl" :to="playerEl">
            <PlayerGestureOverlay :indicator="gestureIndicator" />
        </Teleport>
        <div
            v-if="transcodeActive && !playbackNotice"
            class="transcode-badge"
            :title="
                transcodeMode === 'remux'
                    ? 'The browser cannot read this file container: it is repackaged on the fly'
                    : 'The browser cannot play this format: it is converted on the fly'
            "
        >
            <Icon name="heroicons:arrow-path-rounded-square" size="12" />
            {{ transcodeMode === 'remux' ? 'Remuxed' : 'Converted' }} on the fly
        </div>
        <PlayerPlaybackNotice
            v-if="playbackNotice"
            :notice="playbackNotice"
            :download-url="downloadUrl"
            @retry="transcode.retry()"
        />
    </div>
</template>

<style scoped>
.video-wrapper {
    position: relative;
    width: 100%;
    margin: 0 auto;
    background: #050505;
    overflow: hidden;
}

.transcode-badge {
    position: absolute;
    top: 12px;
    left: 12px;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 3px 8px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 999px;
    background: rgba(8, 8, 8, 0.6);
    backdrop-filter: blur(12px);
    color: rgba(255, 255, 255, 0.7);
    font-size: 10px;
    font-weight: 500;
    pointer-events: auto;
    opacity: 0;
    transition: opacity 0.3s ease;
}

.video-wrapper:hover .transcode-badge {
    opacity: 1;
}

.video-wrapper--portrait {
    max-height: 80vh;
}

:deep(.video-js) {
    font-family: 'Inter', system-ui, sans-serif;
    --primary-color: #ff4d4d;
    --text-color: #ffffff;
}

/* Big Play Button */
:deep(.vjs-big-play-button) {
    background: rgba(255, 77, 77, 0.15);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255, 77, 77, 0.4);
    border-radius: 50%;
    width: 72px;
    height: 72px;
    line-height: 72px;
    margin-left: -36px;
    margin-top: -36px;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow:
        0 0 40px rgba(255, 77, 77, 0.2),
        inset 0 0 20px rgba(255, 77, 77, 0.1);
}

:deep(.vjs-big-play-button:hover) {
    background: rgba(255, 77, 77, 0.25);
    border-color: rgba(255, 77, 77, 0.6);
    transform: scale(1.08);
    box-shadow:
        0 0 60px rgba(255, 77, 77, 0.35),
        inset 0 0 30px rgba(255, 77, 77, 0.15);
}

:deep(.vjs-big-play-button .vjs-icon-placeholder::before) {
    font-size: 36px;
    color: #ff4d4d;
    filter: drop-shadow(0 0 8px rgba(255, 77, 77, 0.5));
}

:deep(.video-js:hover .vjs-big-play-button, .video-js .vjs-big-play-button:focus) {
    border-color: rgba(255, 77, 77, 0.6);
    background-color: rgba(255, 77, 77, 0.1);
    transform: scale(1.05);
    transition: all 0.15s ease;
}

/* ========================================
   FLOATING CONTROL BAR
   ======================================== */
:deep(.vjs-control-bar) {
    position: absolute;
    bottom: 16px;
    left: 16px;
    right: 16px;
    width: auto;
    height: 48px;
    background: rgba(8, 8, 8, 0.85);
    backdrop-filter: blur(20px) saturate(180%);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 14px;
    padding: 4px 6px 0 6px;
    box-shadow:
        0 8px 32px rgba(0, 0, 0, 0.5),
        0 0 0 1px rgba(255, 255, 255, 0.03) inset;
    opacity: 0;
    transform: translateY(8px);
    transition: opacity 0.3s ease;
    display: flex;
    align-items: center;
    gap: 2px;
}

:deep(.video-js:hover .vjs-control-bar),
:deep(.video-js.vjs-user-active .vjs-control-bar),
:deep(.video-js.vjs-paused .vjs-control-bar) {
    opacity: 1;
    transform: translateY(0);
}

/* Override sticky :hover on mobile — hide when user is inactive and video is playing */
:deep(.video-js.vjs-user-inactive:not(.vjs-paused) .vjs-control-bar) {
    opacity: 0;
    transform: translateY(0);
    pointer-events: none;
}

/* Control buttons base styling */
:deep(.vjs-control) {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: rgba(255, 255, 255, 0.7);
    border-radius: 8px;
    flex-shrink: 0;
}

:deep(.vjs-control:hover) {
    color: #ffffff;
}

/* Play/Pause button - slightly larger */
:deep(.vjs-play-control) {
    width: 34px;
    height: 34px;
    margin-right: 4px;
}

:deep(.vjs-play-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-play-control .vjs-icon-placeholder::before) {
    font-size: 22px;
    line-height: 40px;
}

/* Volume Panel */
:deep(.vjs-mute-control) {
    width: 36px;
    height: 36px;
}

:deep(.vjs-mute-control .vjs-icon-placeholder::before) {
    font-size: 18px;
    line-height: 36px;
}

/* Time display */
:deep(.vjs-time-control) {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 11px;
    font-weight: 500;
    line-height: 48px;
    padding: 0 6px;
    color: rgba(255, 255, 255, 0.6);
    min-width: auto;
    flex-shrink: 0;
}

:deep(.vjs-current-time) {
    color: #ffffff;
    margin-right: auto;
    padding-left: 25px;
    padding-right: 25px;
}

:deep(.vjs-time-divider) {
    padding: 0 2px;
    min-width: auto;
    color: rgba(255, 255, 255, 0.3);
}

:deep(.vjs-duration) {
    padding-left: 25px;
    padding-right: 25px;
}

:deep(.vjs-remaining-time) {
    display: none;
}

/* ========================================
   PROGRESS BAR - Above controls
   ======================================== */
:deep(.vjs-progress-control) {
    position: absolute;
    top: -5px;
    left: 12px;
    right: 12px;
    width: auto;
    height: 16px;
    flex: none;
    order: -1;
}

:deep(.vjs-progress-control .vjs-progress-holder) {
    margin: 0;
    height: 4px;
    padding-top: 3px;
    padding-bottom: 3px;
    background-clip: content-box;
    background: rgba(255, 255, 255, 0.12);
    border-radius: 2px;
    transition: height 0.15s ease;
}

:deep(.vjs-progress-control:hover .vjs-progress-holder) {
    height: 6px;
}

:deep(.vjs-progress-control .vjs-play-progress),
:deep(.vjs-progress-control .vjs-load-progress) {
    top: 0px;
    height: 5px;
    border-radius: 2px;
    transition: height 0.15s ease;
}

:deep(.vjs-progress-control:hover .vjs-play-progress),
:deep(.vjs-progress-control:hover .vjs-load-progress) {
    top: 0px;
    height: 6px;
}

:deep(.vjs-play-progress) {
    background: linear-gradient(90deg, #ff4d4d, #ff6b6b);
    box-shadow: 0 0 12px rgba(255, 77, 77, 0.4);
}

:deep(.vjs-play-progress::before) {
    content: '';
    position: absolute;
    right: -6px;
    top: 50%;
    transform: translateY(-50%) scale(0);
    width: 12px;
    height: 12px;
    background: #ffffff;
    border-radius: 50%;
    box-shadow:
        0 0 8px rgba(255, 77, 77, 0.6),
        0 2px 4px rgba(0, 0, 0, 0.3);
    transition: transform 0.15s ease;
}

:deep(.vjs-progress-control:hover .vjs-play-progress::before) {
    transform: translateY(-50%) scale(1);
}

:deep(.vjs-load-progress) {
    background: rgba(255, 255, 255, 0.2);
}

:deep(.vjs-slider) {
    background-color: transparent;
}
:deep(.vjs-slidding) {
    width: 100%;
}

/* Playback Rate */
:deep(.vjs-playback-rate) {
    width: auto;
    min-width: 44px;
}

:deep(.vjs-playback-rate-value) {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 11px;
    font-weight: 600;
    line-height: 36px;
    color: rgba(255, 255, 255, 0.7);
}

:deep(.vjs-playback-rate:hover .vjs-playback-rate-value) {
    color: #ff4d4d;
}

:deep(.vjs-menu) {
    left: 40%;
    transform: translateX(-50%);
    bottom: 75%;
    margin-bottom: 8px;
}

:deep(.vjs-menu-content) {
    background: rgba(8, 8, 8, 0.6) !important;
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 2px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    overflow: hidden;
    width: 48px !important;
}

:deep(.vjs-menu-item) {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 11px;
    padding: 4px 20px;
    border-radius: 6px;
    color: rgba(255, 255, 255, 0.7);
    transition: all 0.1s ease;
}

:deep(.vjs-menu-item:hover) {
    background: rgba(255, 77, 77, 0.15);
    color: #ffffff;
}

:deep(.vjs-menu-item.vjs-selected) {
    background: rgba(255, 77, 77, 0.2);
    color: #ff4d4d;
}

/* PiP, Theater Mode, and Fullscreen */
:deep(.vjs-picture-in-picture-control),
:deep(.vjs-theater-mode-control),
:deep(.vjs-fullscreen-control) {
    width: 36px;
    height: 36px;
}

:deep(.vjs-picture-in-picture-control .vjs-icon-placeholder::before),
:deep(.vjs-fullscreen-control .vjs-icon-placeholder::before) {
    font-size: 18px;
    line-height: 36px;
}

:deep(.vjs-theater-mode-control) {
    display: flex;
    align-items: center;
    justify-content: center;
}

:deep(.vjs-theater-mode-control .vjs-theater-icon) {
    width: 14px;
    height: 14px;
}

:deep(.vjs-theater-mode-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-scrubber-toggle-control) {
    display: flex;
    width: 36px;
    height: 36px;
    align-items: center;
    justify-content: center;
    cursor: pointer;
}

:deep(.vjs-scrubber-toggle-control .vjs-scrubber-icon) {
    width: 15px;
    height: 15px;
}

:deep(.vjs-scrubber-toggle-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-scrubber-toggle-control.vjs-scrubber-toggle-active) {
    color: #ff4d4d;
}

:deep(.vjs-fullscreen-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

/* Playlist Prev/Next Buttons */
:deep(.vjs-playlist-prev-control),
:deep(.vjs-playlist-next-control) {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
}

:deep(.vjs-playlist-prev-control:hover),
:deep(.vjs-playlist-next-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-playlist-next-control) {
    margin-right: 6px;
}

:deep(.vjs-playlist-icon) {
    width: 16px;
    height: 16px;
}

:deep(.vjs-playlist-btn-disabled) {
    opacity: 0.25;
    pointer-events: none;
}

/* Icon alignment fix */
:deep(.vjs-icon-placeholder) {
    display: flex;
    align-items: center;
    justify-content: center;
    transform: none;
}

:deep(.vjs-icon-placeholder::before) {
    position: static;
    display: block;
}

/* ========================================
   THUMBNAIL PREVIEW
   ======================================== */
:deep(.vjs-thumb-preview) {
    position: absolute;
    bottom: 100%;
    margin-bottom: 20px;
    pointer-events: none;
    border: 1px solid rgba(255, 77, 77, 0.3);
    border-radius: 8px;
    overflow: hidden;
    box-shadow:
        0 8px 32px rgba(0, 0, 0, 0.6),
        0 0 20px rgba(255, 77, 77, 0.1);
    z-index: 10;
    background: #0a0a0a;
}

:deep(.vjs-thumb-preview img) {
    display: block;
}

/* ========================================
   MARKER INDICATORS
   ======================================== */
:deep(.vjs-marker-container) {
    position: absolute;
    top: 0px;
    left: 0;
    right: 0;
    height: 4px;
    pointer-events: none;
    z-index: 5;
}

:deep(.vjs-progress-control:hover .vjs-marker-container) {
    top: 0px;
    height: 6px;
}

:deep(.vjs-marker-range) {
    position: absolute;
    top: 0;
    height: 100%;
    background-color: var(--marker-color, #ff4d4d);
    opacity: 0.35;
    pointer-events: none;
}

:deep(.vjs-marker-tick) {
    position: absolute;
    width: 17px;
    height: 17px;
    transform: translate(-50%, -50%);
    top: 50%;
    cursor: pointer;
    pointer-events: auto;
}

:deep(.vjs-marker-tick::before) {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    transform: translate(-50%, -50%);
    background-color: var(--marker-color, #ffffff);
    box-shadow:
        0 0 6px rgba(0, 0, 0, 0.5),
        0 0 12px var(--marker-color, rgba(255, 255, 255, 0.3));
    transition: all 0.15s ease;
}

:deep(.vjs-marker-tick:hover::before) {
    transform: translate(-50%, -50%) scale(1.4);
    box-shadow:
        0 0 8px rgba(0, 0, 0, 0.5),
        0 0 16px var(--marker-color, rgba(255, 255, 255, 0.5));
}

/* Marker Tooltip */
:deep(.vjs-marker-tooltip) {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 20px;
    background: rgba(8, 8, 8, 0.95);
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 77, 77, 0.3);
    border-radius: 10px;
    overflow: hidden;
    width: 280px;
    opacity: 0;
    visibility: hidden;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    pointer-events: none;
    z-index: 20;
    box-shadow:
        0 8px 32px rgba(0, 0, 0, 0.6),
        0 0 20px rgba(255, 77, 77, 0.1);
}

:deep(.vjs-marker-tick:hover .vjs-marker-tooltip) {
    opacity: 1;
    visibility: visible;
}

:deep(.vjs-marker-tooltip-img) {
    width: 280px;
    height: 158px;
    object-fit: cover;
    display: block;
}

:deep(.vjs-marker-tooltip-placeholder) {
    width: 280px;
    height: 158px;
    background: linear-gradient(135deg, rgba(255, 77, 77, 0.08) 0%, rgba(255, 77, 77, 0.03) 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

:deep(.vjs-marker-tooltip-info) {
    padding: 10px 12px;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
}

:deep(.vjs-marker-tooltip-label) {
    font-size: 12px;
    color: white;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-bottom: 4px;
}

:deep(.vjs-marker-tooltip-time) {
    font-size: 11px;
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    color: rgba(255, 255, 255, 0.5);
}

/* ========================================
   REPLAY HEATMAP ("most replayed")
   ======================================== */
:deep(.vjs-replay-heatmap) {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 100%;
    height: 22px;
    pointer-events: none;
    opacity: 0.55;
    transition: opacity 0.15s ease;
    z-index: 1;
}

:deep(.vjs-progress-control:hover .vjs-replay-heatmap) {
    opacity: 1;
}

:deep(.vjs-replay-heatmap svg) {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
}

:deep(.vjs-replay-heatmap-line) {
    stroke: rgba(255, 255, 255, 0.45);
    stroke-width: 1px;
}

/* ========================================
   LOADING SPINNER
   ======================================== */
:deep(.vjs-loading-spinner) {
    border: 6px solid transparent;
    background: rgba(8, 8, 8, 0.6);
    backdrop-filter: blur(8px);
    border-radius: 50%;
    width: 64px;
    height: 64px;
    margin-left: -32px;
    margin-top: -32px;
}

:deep(.vjs-loading-spinner::before) {
    border-color: rgba(255, 77, 77, 0.3);
}

:deep(.vjs-loading-spinner::after) {
    border-top-color: #ff4d4d;
}

/* ========================================
   AB LOOP - Progress Bar Overlay
   ======================================== */
:deep(.vjs-ab-loop-overlay) {
    position: absolute;
    top: 0;
    height: 100%;
    background: rgba(255, 77, 77, 0.2);
    border-left: 2px solid rgba(255, 77, 77, 0.8);
    border-right: 2px solid rgba(255, 77, 77, 0.8);
    border-radius: 2px;
    pointer-events: none;
    z-index: 4;
    box-shadow: 0 0 8px rgba(255, 77, 77, 0.15);
}

:deep(.vjs-ab-loop-marker) {
    position: absolute;
    top: 50%;
    width: 2px;
    height: 100%;
    transform: translateY(-50%);
    background: rgba(255, 184, 77, 0.9);
    pointer-events: none;
    z-index: 4;
    box-shadow: 0 0 6px rgba(255, 184, 77, 0.4);
}

/* ========================================
   AB LOOP - Control Bar Buttons
   ======================================== */

/* Time buttons (A / B) */
:deep(.vjs-ab-loop-time) {
    width: auto !important;
    min-width: 52px;
    height: 28px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 8px;
    cursor: pointer;
    border-radius: 6px;
    border: 1px solid rgba(255, 255, 255, 0.06);
    background: rgba(255, 255, 255, 0.03);
    transition: all 0.15s ease;
    flex-shrink: 0;
}

:deep(.vjs-ab-loop-time:hover) {
    border-color: rgba(255, 77, 77, 0.3);
    background: rgba(255, 77, 77, 0.08);
}

:deep(.vjs-ab-loop-time.vjs-ab-loop-time-active) {
    border-color: rgba(255, 77, 77, 0.3);
    background: rgba(255, 77, 77, 0.08);
}

:deep(.vjs-ab-loop-time.vjs-ab-loop-time-active:hover) {
    border-color: rgba(255, 77, 77, 0.5);
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-ab-loop-label) {
    font-family: 'Inter', system-ui, sans-serif;
    font-size: 10px;
    font-weight: 700;
    color: rgba(255, 77, 77, 0.7);
    line-height: 1;
    flex-shrink: 0;
}

:deep(.vjs-ab-loop-time-active .vjs-ab-loop-label) {
    color: #ff4d4d;
}

:deep(.vjs-ab-loop-value) {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 10px;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.5);
    line-height: 1;
    white-space: nowrap;
}

:deep(.vjs-ab-loop-time-active .vjs-ab-loop-value) {
    color: rgba(255, 255, 255, 0.8);
}

/* Loop toggle button */
:deep(.vjs-ab-loop-toggle) {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    border-radius: 8px;
    transition: all 0.15s ease;
}

:deep(.vjs-ab-loop-icon) {
    width: 14px;
    height: 14px;
}

:deep(.vjs-ab-loop-toggle:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
}

:deep(.vjs-ab-loop-toggle.vjs-ab-loop-enabled) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.15);
}

:deep(.vjs-ab-loop-toggle.vjs-ab-loop-enabled:hover) {
    background: rgba(255, 77, 77, 0.25);
}

/* ========================================
   TOUCH GESTURES
   ======================================== */
/* Inline, the browser keeps vertical scrolling and pinch-zoom; horizontal drags are scrubs */
:deep(.vjs-gestures .vjs-tech) {
    touch-action: pan-y pinch-zoom;
    -webkit-touch-callout: none;
    -webkit-user-select: none;
    user-select: none;
}

/* In fullscreen, every drag is a gesture (vertical: volume and brightness) */
:deep(.vjs-gestures.vjs-fullscreen .vjs-tech) {
    touch-action: none;
}

/* Hide text track display from control bar area */
:deep(.vjs-text-track-display) {
    bottom: 80px;
}

:deep(.vjs-fullscreen .vjs-text-track-display) {
    bottom: 100px;
}
</style>
