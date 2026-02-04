import type videojs from 'video.js';
import type { VttCue } from './useVttParser';

type Player = ReturnType<typeof videojs>;

export function useThumbnailPreview(player: Ref<Player | null>, vttCues: Ref<VttCue[]>) {
    function setup() {
        if (!player.value) return;

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const progressControl = (player.value as unknown as Record<string, any>).controlBar
            ?.progressControl;
        if (!progressControl) return;

        const seekBar = progressControl.seekBar;
        if (!seekBar) return;

        const thumbEl = document.createElement('div');
        thumbEl.className = 'vjs-thumb-preview';
        thumbEl.style.display = 'none';
        seekBar.el().appendChild(thumbEl);

        const imgEl = document.createElement('img');
        imgEl.style.display = 'block';
        thumbEl.appendChild(imgEl);

        let currentSpriteUrl = '';

        const onMouseMove = (e: Event) => {
            if (vttCues.value.length === 0) return;

            const mouseEvent = e as MouseEvent;

            // Don't show sprite preview when hovering a marker tick
            const target = mouseEvent.target as HTMLElement;
            if (target.closest('.vjs-marker-tick')) {
                thumbEl.style.display = 'none';
                return;
            }

            const seekBarRect = seekBar.el().getBoundingClientRect();
            const percent = (mouseEvent.clientX - seekBarRect.left) / seekBarRect.width;
            const duration = player.value!.duration();
            if (!duration) return;

            const time = percent * duration;
            const cue = vttCues.value.find((c) => time >= c.start && time < c.end);
            if (!cue) {
                thumbEl.style.display = 'none';
                return;
            }

            thumbEl.style.display = 'block';

            if (currentSpriteUrl !== cue.url) {
                imgEl.src = cue.url;
                currentSpriteUrl = cue.url;
            }

            imgEl.style.objectFit = 'none';
            imgEl.style.objectPosition = `-${cue.x}px -${cue.y}px`;
            imgEl.style.width = `${cue.w}px`;
            imgEl.style.height = `${cue.h}px`;

            thumbEl.style.width = `${cue.w}px`;
            thumbEl.style.height = `${cue.h}px`;

            const thumbLeft = mouseEvent.clientX - seekBarRect.left - cue.w / 2;
            const clampedLeft = Math.max(0, Math.min(thumbLeft, seekBarRect.width - cue.w));
            thumbEl.style.left = `${clampedLeft}px`;
        };

        const onMouseOut = () => {
            thumbEl.style.display = 'none';
        };

        const progressControlEl = document.querySelector('.vjs-progress-control');
        if (!progressControlEl) return;
        progressControlEl.addEventListener('mousemove', onMouseMove);
        progressControlEl.addEventListener('mouseout', onMouseOut);
    }

    return { setup };
}
