import type videojs from 'video.js';

type Player = ReturnType<typeof videojs>;

/**
 * Compact layout of the scene player controls (utils/playerCompactLayout.ts): when the player
 * is narrower than COMPACT_PLAYER_WIDTH, the controls are spread over it like on the mobile
 * players: the secondary ones (speed, A/B loop, picture-in-picture, scrubber) in a bar at the
 * top, play with the playlist previous/next in the middle, and the progress, mute, time and
 * fullscreen in the bottom bar. The theater mode is hidden (no use on a narrow screen). The
 * video.js components are untouched, only their elements move; they go back to the control bar,
 * in their order, when the player gets wide again.
 *
 * Call attach() once the control bar has all its children (after the playlist buttons).
 */
export function usePlayerCompactLayout(player: Ref<Player | null>) {
    const compact = ref(false);

    let root: HTMLElement | null = null;
    let bar: HTMLElement | null = null;
    let top: HTMLElement | null = null;
    let center: HTMLElement | null = null;
    let original: Element[] = [];
    let observer: ResizeObserver | null = null;

    function apply(on: boolean) {
        if (!root || !bar || !top || !center || on === compact.value) return;
        compact.value = on;
        root.classList.toggle('vjs-compact', on);
        if (on) {
            const moved = original
                .map((el) => ({ el, slot: compactSlotOf(el.classList) }))
                .filter((m) => m.slot !== 'bottom')
                .sort((a, b) => compactOrderOf(a.el.classList) - compactOrderOf(b.el.classList));
            for (const { el, slot } of moved) (slot === 'top' ? top : center).appendChild(el);
        } else {
            for (const el of original) bar.appendChild(el);
        }
    }

    function attach() {
        const p = player.value;
        if (!p || root) return;
        root = p.el() as HTMLElement;
        bar = root.querySelector<HTMLElement>(':scope > .vjs-control-bar');
        if (!bar) return;
        original = Array.from(bar.children);

        top = document.createElement('div');
        top.className = 'vjs-compact-top';
        // the touch gestures leave its taps to the buttons
        top.setAttribute('data-player-overlay', '');
        center = document.createElement('div');
        center.className = 'vjs-compact-center';
        root.appendChild(top);
        root.appendChild(center);

        observer = new ResizeObserver(([entry]) => {
            if (entry) apply(isCompactPlayer(entry.contentRect.width));
        });
        observer.observe(root);
    }

    function detach() {
        observer?.disconnect();
        observer = null;
        apply(false);
        top?.remove();
        center?.remove();
        root = bar = top = center = null;
        original = [];
    }

    return { compact, attach, detach };
}
