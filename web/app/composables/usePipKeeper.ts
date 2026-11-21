import type videojs from 'video.js';

type Player = ReturnType<typeof videojs>;

interface KeptPlayer {
    player: Player;
    sceneUrl: string;
    onReturn: (time: number) => void;
}

export interface PipHandoff {
    sceneUrl: string;
    time: number;
    playing: boolean;
}

const HOST_ID = 'goonhub-pip-keeper';
// Leaving picture-in-picture this soon after the move means the browser closed it because of the
// move itself, not that the user asked to go back
const SETTLE_MS = 1000;

// A single player kept alive in picture-in-picture after its page was left (module state, shared
// by every ScenePlayer)
let kept: KeptPlayer | null = null;

function keeperHost(): HTMLElement {
    let host = document.getElementById(HOST_ID);
    if (!host) {
        host = document.createElement('div');
        host.id = HOST_ID;
        host.setAttribute('aria-hidden', 'true');
        Object.assign(host.style, {
            position: 'fixed',
            left: '0',
            top: '0',
            width: '1px',
            height: '1px',
            overflow: 'hidden',
            opacity: '0',
            pointerEvents: 'none',
            zIndex: '-1',
        });
        document.body.appendChild(host);
    }
    return host;
}

// Element.moveBefore keeps the media state (playback, picture-in-picture) during the move where
// supported; appendChild is the fallback (some browsers then pause or leave picture-in-picture)
function moveInto(parent: HTMLElement, el: Element) {
    const moveBefore = (
        parent as unknown as { moveBefore?: (node: Node, child: Node | null) => void }
    ).moveBefore;
    if (typeof moveBefore === 'function') {
        try {
            moveBefore.call(parent, el, null);
            return;
        } catch {
            // fall back to appendChild
        }
    }
    parent.appendChild(el);
}

/** True when the player's video is the document's picture-in-picture element. */
export function isPlayerInPictureInPicture(p: Player): boolean {
    if (typeof document === 'undefined') return false;
    const pipEl = document.pictureInPictureElement;
    return !!pipEl && (p.el() as HTMLElement).contains(pipEl);
}

function release(entry: KeptPlayer) {
    if (kept === entry) kept = null;
    if (!entry.player.isDisposed()) entry.player.dispose();
}

/**
 * Keeps a scene playing in picture-in-picture when its page is left: the player element moves
 * into a hidden host outside the page, and is disposed when picture-in-picture ends. When the
 * user goes back to the tab from the picture-in-picture window while it plays, onReturn gets the
 * current time (to reopen the watch page there). Returns false (the caller disposes the player)
 * when the player is not in picture-in-picture.
 */
function keep(p: Player, sceneUrl: string, onReturn: (time: number) => void): boolean {
    if (!isPlayerInPictureInPicture(p)) return false;
    takeOver();

    const entry: KeptPlayer = { player: p, sceneUrl, onReturn };
    kept = entry;
    const keptAt = performance.now();
    const wasPlaying = !p.paused();
    moveInto(keeperHost(), p.el());

    p.on('leavepictureinpicture', () => {
        if (kept !== entry) return;
        const goBack = performance.now() - keptAt > SETTLE_MS && !p.paused();
        const time = p.currentTime() ?? 0;
        release(entry);
        if (goBack) entry.onReturn(time);
    });

    // After a fallback move the browser may have left picture-in-picture or paused the video
    setTimeout(() => {
        if (kept !== entry) return;
        if (!isPlayerInPictureInPicture(p)) {
            release(entry);
            return;
        }
        if (wasPlaying && p.paused()) p.play()?.catch(() => {});
    }, 0);
    return true;
}

/**
 * Ends the kept picture-in-picture player, if any (a new player is opening), and returns where it
 * was so that the new player can continue the same scene.
 */
function takeOver(): PipHandoff | null {
    const entry = kept;
    if (!entry) return null;
    kept = null;
    const p = entry.player;
    if (p.isDisposed()) return null;
    const handoff: PipHandoff = {
        sceneUrl: entry.sceneUrl,
        time: p.currentTime() ?? 0,
        playing: !p.paused(),
    };
    if (isPlayerInPictureInPicture(p)) document.exitPictureInPicture().catch(() => {});
    p.dispose();
    return handoff;
}

export function usePipKeeper() {
    return { keep, takeOver };
}
