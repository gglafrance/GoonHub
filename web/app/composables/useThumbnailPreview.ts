import type videojs from 'video.js';
import type { VttCue } from './useVttParser';

type Player = ReturnType<typeof videojs>;

export function useThumbnailPreview(player: Ref<Player | null>, vttCues: Ref<VttCue[]>) {
    function setup() {
        if (!player.value) return;

        const progressControl = player.value.controlBar?.progressControl;
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

        const onMouseMove = (e: MouseEvent) => {
            if (vttCues.value.length === 0) return;

            const seekBarRect = seekBar.el().getBoundingClientRect();
            const percent = (e.clientX - seekBarRect.left) / seekBarRect.width;
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

            const thumbLeft = e.clientX - seekBarRect.left - cue.w / 2;
            const clampedLeft = Math.max(0, Math.min(thumbLeft, seekBarRect.width - cue.w));
            thumbEl.style.left = `${clampedLeft}px`;
        };

        const onMouseOut = () => {
            thumbEl.style.display = 'none';
        };

        seekBar.el().addEventListener('mousemove', onMouseMove);
        seekBar.el().addEventListener('mouseout', onMouseOut);
    }

    return { setup };
}
