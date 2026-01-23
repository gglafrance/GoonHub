<script setup lang="ts">
import 'videojs-video-element';
import 'media-chrome';
import 'media-chrome/menu';
import 'media-chrome/media-theme-element';
import { onMounted, onUnmounted, ref, watch } from 'vue';
import type { Video } from '~/types/video';

const props = defineProps<{
    videoUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    video?: Video;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    error: [error: any];
}>();

const thumbnailsTrackSrc = ref<string | null>(null);

const formatVttTime = (seconds: number): string => {
    const date = new Date(0);
    date.setSeconds(seconds);
    date.setMilliseconds((seconds % 1) * 1000);
    return date.toISOString().substr(11, 12);
};

const generateVttBlob = (video: Video): string | null => {
    if (!video.frame_paths || !video.frame_interval || video.frame_paths.length === 0) return null;

    const frames = video.frame_paths.split(',');
    if (frames.length === 0) return null;

    let vttContent = 'WEBVTT\n\n';
    let currentTime = 0;
    const interval = video.frame_interval;

    frames.forEach((frame) => {
        const start = formatVttTime(currentTime);
        const end = formatVttTime(currentTime + interval);
        // Use absolute URL to avoid "Invalid URL" errors in media-chrome
        const url = new URL(`/frames/${video.id}/${frame}`, window.location.origin).toString();

        vttContent += `${start} --> ${end}\n${url}\n\n`;
        currentTime += interval;
    });

    const blob = new Blob([vttContent], { type: 'text/vtt' });
    return URL.createObjectURL(blob);
};

watch(
    () => props.video,
    (newVideo) => {
        if (thumbnailsTrackSrc.value) {
            URL.revokeObjectURL(thumbnailsTrackSrc.value);
            thumbnailsTrackSrc.value = null;
        }

        if (newVideo) {
            thumbnailsTrackSrc.value = generateVttBlob(newVideo);
        }
    },
    { immediate: true, deep: true },
);

onUnmounted(() => {
    if (thumbnailsTrackSrc.value) {
        URL.revokeObjectURL(thumbnailsTrackSrc.value);
    }
});

const mediaThemeRef = ref<HTMLElement | null>(null);

onMounted(() => {
    const template = document.createElement('template');
    template.innerHTML = `
      <!-- Sutro-inspired Theme for GoonHub -->
      <style>
        :host {
          --_primary-color: var(--media-primary-color, #fff);
          --_secondary-color: var(--media-secondary-color, transparent);
          --_accent-color: var(--media-accent-color, #2ecc71);
          display: block;
          width: 100%;
          height: 100%;
        }

        media-controller {
          --base: 14px;
          width: 100%;
          height: 100%;
          background: #000;

          font-size: calc(1 * var(--base));
          font-family: 'Inter', system-ui, sans-serif;
          --media-font-family: 'Inter', system-ui, sans-serif;
          -webkit-font-smoothing: antialiased;

          --media-primary-color: #fff;
          --media-secondary-color: transparent;
          --media-menu-background: rgba(15, 15, 15, 0.9);
          --media-text-color: var(--_primary-color);
          --media-control-hover-background: var(--media-secondary-color);

          --media-range-track-height: calc(0.3 * var(--base));
          --media-range-thumb-height: var(--base);
          --media-range-thumb-width: var(--base);
          --media-range-thumb-border-radius: var(--base);

          --media-control-height: calc(1.5 * var(--base));
        }

        media-controller[breakpointmd] {
          --base: 16px;
        }

        media-controller[mediaisfullscreen] {
          --base: 20px;
        }

        .media-button {
          --media-control-hover-background: var(--_secondary-color);
          --media-tooltip-background: rgb(28 28 28 / .9);
          --media-text-content-height: 1.2;
          --media-tooltip-padding: .7em 1em;
          --media-tooltip-distance: 8px;
          --media-tooltip-container-margin: 18px;
          position: relative;
          padding: 6px;
          opacity: 0.9;
          transition: opacity 0.1s cubic-bezier(0.4, 0, 1, 1);
        }
        
        .media-button:hover {
            opacity: 1;
            color: var(--_accent-color);
            --media-icon-color: var(--_accent-color);
        }

        .media-button svg {
          fill: none;
          stroke: currentColor;
          stroke-width: 2;
          stroke-linecap: round;
          stroke-linejoin: round;
          width: 100%;
          height: 100%;
        }
        
        /* Specific override for SVG icons that use fill instead of stroke or specific paths */
        .media-button svg path {
            stroke: currentColor;
        }
      </style>

      <media-controller
        breakpoints="md:480"
        hotkeys
        defaultstreamtype="on-demand"
      >
        <slot name="media" slot="media"></slot>
        <slot name="poster" slot="poster"></slot>
        <slot name="centered-chrome" slot="centered-chrome"></slot>
        <media-error-dialog slot="dialog"></media-error-dialog>

        <!-- Controls Gradient -->
        <style>
          .media-gradient-bottom {
            position: absolute;
            bottom: 0;
            width: 100%;
            height: calc(12 * var(--base));
            pointer-events: none;
            background: linear-gradient(to top, rgba(0,0,0,0.9) 0%, rgba(0,0,0,0) 100%);
          }
        </style>
        <div class="media-gradient-bottom"></div>

        <!-- Settings Menu -->
        <style>
          media-settings-menu {
            --media-menu-icon-height: 20px;
            --media-menu-item-icon-height: 20px;
            --media-settings-menu-min-width: calc(10 * var(--base));
            padding-block: calc(0.5 * var(--base));
            margin-right: 10px;
            margin-bottom: 70px; /* Push up above control bar */
            border-radius: 12px;
            border: 1px solid rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(12px);
            z-index: 100;
          }

          media-settings-menu-item,
          [role='menu']::part(menu-item) {
            --media-icon-color: var(--_primary-color);
            margin-inline: calc(0.45 * var(--base));
            height: calc(2 * var(--base));
            font-size: calc(0.8 * var(--base));
            border-radius: 6px;
          }
          
          media-settings-menu-item:hover {
            color: var(--_accent-color);
            background: rgba(255,255,255,0.1);
          }
        </style>
        <media-settings-menu hidden anchor="auto">
          <media-settings-menu-item>
            Playback Speed
            <media-playback-rate-menu slot="submenu" hidden>
              <div slot="title">Playback Speed</div>
            </media-playback-rate-menu>
          </media-settings-menu-item>
        </media-settings-menu>

        <!-- Control Bar -->
        <style>
          media-control-bar {
            position: absolute;
            height: calc(4 * var(--base));
            bottom: 0;
            left: 0;
            right: 0;
            padding: 0 var(--base);
            display: flex;
            align-items: center;
            gap: 10px;
          }
        </style>
        <media-control-bar>
          <!-- Play/Pause -->
          <media-play-button class="media-button">
          </media-play-button>

          <!-- Volume -->
          <media-mute-button class="media-button"></media-mute-button>
          <media-volume-range style="width: 100px; --media-range-bar-color: var(--_accent-color);"></media-volume-range>

          <!-- Time Display -->
          <media-time-display style="margin-left: 10px;"></media-time-display>
          
          <!-- Time Range (Progress) -->
          <style>
             media-time-range {
                 flex-grow: 1;
                 --media-range-bar-color: var(--_accent-color);
                 --media-range-track-background: rgba(255,255,255,0.2);
                 --media-time-buffered-color: rgba(255,255,255,0.4);
                 --media-range-thumb-opacity: 0;
             }
             media-time-range:hover {
                 --media-range-thumb-opacity: 1;
             }
          </style>
          <media-time-range>
             <media-preview-thumbnail slot="preview" style="border: 2px solid var(--_accent-color); border-radius: 8px; max-width: 180px;"></media-preview-thumbnail>
             <media-preview-time-display slot="preview"></media-preview-time-display>
          </media-time-range>

          <media-time-display showduration style="margin-right: 10px;"></media-time-display>

          <!-- Settings -->
          <media-settings-menu-button class="media-button"></media-settings-menu-button>

          <!-- PIP -->
          <media-pip-button class="media-button"></media-pip-button>
          
          <!-- Fullscreen -->
          <media-fullscreen-button class="media-button"></media-fullscreen-button>
        </media-control-bar>
      </media-controller>
    `;

    if (mediaThemeRef.value) {
        // @ts-ignore
        mediaThemeRef.value.template = template;
    }
});
</script>

<template>
    <div class="video-wrapper">
        <media-theme ref="mediaThemeRef" class="video-theme">
            <videojs-video
                slot="media"
                :key="videoUrl"
                :poster="posterUrl"
                :autoplay="autoplay"
                playsinline
                crossorigin="anonymous"
                class="video-element"
                @play="emit('play')"
                @pause="emit('pause')"
                @error="emit('error', $event)"
            >
                <source :src="videoUrl" type="video/mp4" />
                <track
                    v-if="thumbnailsTrackSrc"
                    default
                    kind="metadata"
                    label="thumbnails"
                    :src="thumbnailsTrackSrc"
                />
            </videojs-video>
        </media-theme>
    </div>
</template>

<style scoped>
.video-wrapper {
    width: 100%;
    /* Default aspect ratio, but allow it to be overridden or grow if container specifies height */
    aspect-ratio: 16/9;
    height: 100%;
    background: #000;
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.5);
}

.video-theme {
    width: 100%;
    height: 100%;
    --media-accent-color: #2ecc71;
    --media-primary-color: #fff;
    --media-secondary-color: rgba(46, 204, 113, 0.2);
}

/* Ensure video element fills the container */
.video-element {
    width: 100%;
    height: 100%;
}
</style>
