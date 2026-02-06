<script setup lang="ts">
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Scene } from '~/types/scene';
import type { Marker } from '~/types/marker';

type Player = ReturnType<typeof videojs>;

// Theater mode button interface for type safety
interface TheaterModeButtonInstance {
    setTheaterMode: (active: boolean) => void;
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
}

const props = defineProps<{
    sceneUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    loop?: boolean;
    defaultVolume?: number;
    scene?: Scene;
    startTime?: number;
    markers?: Marker[];
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    ended: [];
    error: [error: unknown];
    viewRecorded: [];
}>();

const videoElement = ref<HTMLVideoElement>();
const player = shallowRef<Player | null>(null);
const settingsStore = useSettingsStore();
const theaterModeButton = shallowRef<TheaterModeButtonInstance | null>(null);
const { vttCues, loadVttCues } = useVttParser();
const { setup: setupThumbnailPreview } = useThumbnailPreview(player, vttCues);
const {
    setup: setupMarkerIndicators,
    cleanup: cleanupMarkerIndicators,
    update: updateMarkerIndicators,
} = useMarkerIndicators(
    player,
    computed(() => props.markers ?? []),
);

const sceneRef = computed(() => props.scene);
const { hasRecordedView, setupTracking, cleanup } = useWatchTracking({
    player,
    scene: sceneRef,
});

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

    player.value = videojs(videoElement.value, {
        controls: true,
        autoplay: props.autoplay ? 'any' : false,
        loop: props.loop ?? false,
        preload: 'metadata',
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
                'pipToggle',
                'TheaterModeButton',
                'fullscreenToggle',
            ],
        },
    });

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
    }

    // Set initial volume (video.js uses 0-1 range)
    const volume = props.defaultVolume != null ? props.defaultVolume / 100 : 1;
    player.value.volume(volume);

    player.value.on('play', () => emit('play'));
    player.value.on('pause', () => emit('pause'));
    player.value.on('ended', () => emit('ended'));
    player.value.on('error', (e: unknown) => emit('error', e));

    // Set up watch tracking (handles timeupdate, ended, and beforeunload)
    setupTracking();

    // Emit viewRecorded when first recorded
    watch(hasRecordedView, (recorded) => {
        if (recorded) emit('viewRecorded');
    });

    player.value.ready(() => {
        setupThumbnailPreview();
        setupMarkerIndicators();
        if (vttUrl.value) {
            loadVttCues(vttUrl.value);
        }

        // Seek to start time if provided
        if (props.startTime && props.startTime > 0) {
            player.value!.currentTime(props.startTime);
        }

        if (props.autoplay) {
            player.value!.play()?.catch(() => {
                // Browser blocked unmuted autoplay — retry muted
                player.value!.muted(true);
                player.value!.play();
            });
        }
    });
});

watch(
    () => props.sceneUrl,
    () => {
        if (player.value) {
            player.value.src({ type: 'video/mp4', src: props.sceneUrl });
            if (props.autoplay) {
                player.value.play()?.catch(() => {
                    player.value!.muted(true);
                    player.value!.play();
                });
            }
        }
    },
);

watch(vttUrl, (newVttUrl) => {
    if (newVttUrl) {
        loadVttCues(newVttUrl);
    }
});

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
            player.value.play()?.catch(() => {
                player.value!.muted(true);
                player.value!.play();
            });
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

defineExpose({
    getCurrentTime: () => player.value?.currentTime() ?? 0,
    player,
    vttCues,
});

onBeforeUnmount(() => {
    cleanup();
    cleanupMarkerIndicators();
    if (player.value) {
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
            <source :src="sceneUrl" type="video/mp4" />
        </video>
    </div>
</template>

<style scoped>
.video-wrapper {
    width: 100%;
    margin: 0 auto;
    background: #050505;
    overflow: hidden;
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

:deep(.vjs-fullscreen-control:hover) {
    color: #ff4d4d;
    background: rgba(255, 77, 77, 0.12);
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

/* Hide text track display from control bar area */
:deep(.vjs-text-track-display) {
    bottom: 80px;
}

:deep(.vjs-fullscreen .vjs-text-track-display) {
    bottom: 100px;
}
</style>
