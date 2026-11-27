// Requests that write but do not change what the lists show (watch tracking, auth)
const NON_DATA_WRITES: RegExp[] = [
    /^\/api\/v1\/auth\//,
    /^\/api\/v1\/scenes\/\d+\/watch$/,
    /^\/api\/v1\/playlists\/[^/]+\/progress$/,
];

/**
 * Whether a request changes library data, i.e. lists cached for Back may now be stale.
 *
 * @param method - HTTP method (GET when omitted)
 * @param url - request URL, absolute or relative to the app origin
 * @param origin - the app origin, to ignore requests to other hosts
 */
export function isDataMutation(method: string | undefined, url: string, origin: string): boolean {
    const m = (method || 'GET').toUpperCase();
    if (m === 'GET' || m === 'HEAD' || m === 'OPTIONS') return false;
    let parsed: URL;
    try {
        parsed = new URL(url, origin);
    } catch {
        return false;
    }
    if (parsed.origin !== origin || !parsed.pathname.startsWith('/api/')) return false;
    return !NON_DATA_WRITES.some((re) => re.test(parsed.pathname));
}
