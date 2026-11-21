import type { RouterConfig } from '@nuxt/schema';
import type { RouteLocationNormalized } from 'vue-router';
import { START_LOCATION } from 'vue-router';

const samePath = (a: RouteLocationNormalized, b: RouteLocationNormalized) =>
    a.path.replace(/\/$/, '') === b.path.replace(/\/$/, '');

/** Waits for the new page to be mounted (with a fallback if the hook does not come). */
function pageMounted(): Promise<void> {
    const nuxtApp = useNuxtApp();
    const hook = nuxtApp._runningTransition ? 'page:transition:finish' : 'page:loading:end';
    return new Promise((resolve) => {
        const fallback = setTimeout(resolve, 1000);
        nuxtApp.hooks.hookOnce(hook, () => {
            clearTimeout(fallback);
            nextFrame(resolve);
        });
    });
}

/**
 * Scroll handling:
 * - Back/Forward restore the saved position, once the list data is rendered (the pages load
 *   their data after mounting, so the saved position is not reachable at once); a reload is
 *   handled by plugins/scroll-restore.client.ts;
 * - a new navigation to another page starts at the top;
 * - a query change on the same page (filters, pagination, tabs) keeps the position, the
 *   pagination scrolls up by itself.
 */
export default <RouterConfig>{
    scrollBehavior(to, from, savedPosition) {
        const router = useRouter();
        // Query-only replaces (state completed by the page) do not cancel the restore
        const isCurrent = () => router.currentRoute.value.path === to.path;
        // Nuxt calls this first for the first navigation after the load, with START_LOCATION as
        // `from` and the load position as savedPosition when there is one
        const fromKnown = from !== START_LOCATION;
        const sameAsFrom = fromKnown && samePath(to, from);

        if (sameAsFrom && !savedPosition) {
            if (to.hash && to.hash !== from.hash) return { el: to.hash, behavior: 'smooth' };
            if (from.hash && !to.hash) return { left: 0, top: 0 };
            return false;
        }

        let position = savedPosition;
        if (!fromKnown && position && isLoadScrollPosition(position)) position = null;

        if (position) {
            const target = { left: position.left ?? 0, top: position.top ?? 0 };
            if (target.top <= 0) return target;
            const wait = sameAsFrom ? Promise.resolve() : pageMounted();
            return wait.then(() => waitForScrollTarget(target, { isCurrent }));
        }

        return pageMounted().then(() => {
            if (!isCurrent()) return false;
            return to.hash ? { el: to.hash } : { left: 0, top: 0 };
        });
    },
};
