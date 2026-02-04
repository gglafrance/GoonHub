import type videojs from 'video.js';
import type { Marker } from '~/types/marker';

type Player = ReturnType<typeof videojs>;

export function useMarkerIndicators(
    player: Ref<Player | null>,
    markers: Ref<Marker[]>,
    onSeek?: (timestamp: number) => void,
) {
    let container: HTMLDivElement | null = null;
    let boundUpdate: (() => void) | null = null;

    function setup() {
        if (!player.value) return;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const progressHolder = (
            player.value as unknown as Record<string, any>
        ).controlBar?.progressControl?.seekBar?.el();
        if (!progressHolder) return;

        // Create container for marker ticks
        container = document.createElement('div');
        container.className = 'vjs-marker-container';
        progressHolder.appendChild(container);

        // Bind update function for event listener
        boundUpdate = () => update();

        // Listen for when duration becomes available
        player.value.on('loadedmetadata', boundUpdate);
        player.value.on('durationchange', boundUpdate);

        // Try to update immediately in case metadata is already loaded
        update();
    }

    function cleanup() {
        // Remove event listeners
        if (player.value && boundUpdate) {
            player.value.off('loadedmetadata', boundUpdate);
            player.value.off('durationchange', boundUpdate);
        }
        boundUpdate = null;

        if (container && container.parentNode) {
            container.parentNode.removeChild(container);
        }
        container = null;
    }

    function update() {
        if (!container || !player.value) return;

        const duration = player.value.duration();
        if (!duration || duration <= 0) return;

        // Clear existing ticks
        container.innerHTML = '';

        // Create tick for each marker
        markers.value.forEach((marker) => {
            const percent = (marker.timestamp / duration) * 100;
            if (percent < 0 || percent > 100) return;

            const tick = createTick(marker, percent);
            container!.appendChild(tick);
        });
    }

    function createTick(marker: Marker, percent: number): HTMLElement {
        const tick = document.createElement('div');
        tick.className = 'vjs-marker-tick';
        tick.style.left = `${percent}%`;
        // Use marker's color (fallback to lava red accent)
        tick.style.setProperty('--marker-color', marker.color || '#FF4D4D');

        // Create tooltip
        const tooltip = document.createElement('div');
        tooltip.className = 'vjs-marker-tooltip';

        // Thumbnail (or placeholder)
        if (marker.thumbnail_path) {
            const img = document.createElement('img');
            img.src = `/marker-thumbnails/${marker.id}`;
            img.alt = marker.label || 'Marker';
            img.className = 'vjs-marker-tooltip-img';
            tooltip.appendChild(img);
        } else {
            const placeholder = document.createElement('div');
            placeholder.className = 'vjs-marker-tooltip-placeholder';
            tooltip.appendChild(placeholder);
        }

        // Info container for label and timestamp
        const info = document.createElement('div');
        info.className = 'vjs-marker-tooltip-info';

        // Label
        if (marker.label) {
            const label = document.createElement('div');
            label.className = 'vjs-marker-tooltip-label';
            label.textContent = marker.label;
            info.appendChild(label);
        }

        // Timestamp
        const timestamp = document.createElement('div');
        timestamp.className = 'vjs-marker-tooltip-time';
        timestamp.textContent = formatTime(marker.timestamp);
        info.appendChild(timestamp);

        tooltip.appendChild(info);

        tick.appendChild(tooltip);

        // Click handler
        tick.addEventListener('click', (e) => {
            e.stopPropagation();
            if (onSeek) {
                onSeek(marker.timestamp);
            } else if (player.value) {
                player.value.currentTime(marker.timestamp);
                player.value.play()?.catch(() => {});
            }
        });

        return tick;
    }

    function formatTime(seconds: number): string {
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    return { setup, cleanup, update };
}
