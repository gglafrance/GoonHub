const STORAGE_KEY = 'scroll-on-unload';

interface UnloadScroll {
    position: number | null;
    href: string;
    left: number;
    top: number;
}

/**
 * Scroll position across a reload: saved when the page is left, restored once the list data of
 * the reloaded page is rendered (the browser's own restoration runs before that and gives up).
 * Kept in sessionStorage, as the router rewrites the history state of the first navigation.
 */
export default defineNuxtPlugin((nuxtApp) => {
    const router = useRouter();
    const currentPosition = () => {
        const pos = window.history.state?.position;
        return typeof pos === 'number' ? pos : null;
    };

    let saved: UnloadScroll | null = null;
    try {
        saved = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || 'null');
        sessionStorage.removeItem(STORAGE_KEY);
    } catch {
        saved = null;
    }

    if (
        saved &&
        saved.top > 0 &&
        saved.href === window.location.href &&
        saved.position === currentPosition()
    ) {
        const target = { left: saved.left, top: saved.top };
        setLoadScrollPosition(target);
        const path = router.currentRoute.value.path;
        nuxtApp.hooks.hookOnce('app:suspense:resolve', () => {
            waitForScrollTarget(target, {
                isCurrent: () => router.currentRoute.value.path === path,
            }).then((position) => {
                if (position) window.scrollTo(position.left, position.top);
            });
        });
    }

    window.addEventListener('pagehide', () => {
        const value: UnloadScroll = {
            position: currentPosition(),
            href: window.location.href,
            left: window.scrollX,
            top: window.scrollY,
        };
        try {
            sessionStorage.setItem(STORAGE_KEY, JSON.stringify(value));
        } catch {
            // Storage disabled: no restore after a reload
        }
    });
});
