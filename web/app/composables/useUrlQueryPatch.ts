import type { LocationQuery, RouteLocationNormalizedLoaded, Router } from 'vue-router';
import type { QueryPatch } from '~/utils/urlQuery';

interface PendingQuery {
    path: string;
    query: LocationQuery;
}

// Shared by every caller: patches made in the same tick are merged into one router.replace,
// and a patch made while a replace is in flight builds on it instead of on the stale route.
let pending: PendingQuery | null = null;
let inflight: PendingQuery | null = null;
let scheduled = false;

function flush(router: Router) {
    scheduled = false;
    const target = pending;
    pending = null;
    if (!target) return;
    const current = router.currentRoute.value;
    // The user left the page in the meantime: never write its state on another page
    if (current.path !== target.path) return;
    if (sameQuery(current.query, target.query)) return;
    inflight = target;
    router.replace({ query: target.query, hash: current.hash }).finally(() => {
        if (inflight === target) inflight = null;
    });
}

/**
 * Writes list state into the URL query (history replace, no new entry). Safe to call from
 * several watchers in the same tick: the patches are merged instead of overwriting each other.
 *
 * @param route - the route of the calling page (useRoute()), so that a late write never lands on
 *   the next page
 */
export function patchRouteQuery(
    router: Router,
    route: RouteLocationNormalizedLoaded,
    patch: QueryPatch,
) {
    const current = router.currentRoute.value;
    if (route.path !== current.path) return;
    let base: LocationQuery = current.query;
    if (pending?.path === current.path) base = pending.query;
    else if (inflight?.path === current.path) base = inflight.query;
    pending = { path: current.path, query: mergeQuery(base, patch) };
    if (!scheduled) {
        scheduled = true;
        queueMicrotask(() => flush(router));
    }
}

/** patchRouteQuery bound to the current page. */
export function useUrlQueryPatch() {
    const route = useRoute();
    const router = useRouter();
    return (patch: QueryPatch) => patchRouteQuery(router, route, patch);
}
