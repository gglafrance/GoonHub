/**
 * Marks the in-memory list data as stale after each successful request that changes library
 * data, whatever the API composable that sent it.
 */
export default defineNuxtPlugin(() => {
    const { bump } = useDataVersion();
    const originalFetch = window.fetch.bind(window);
    const origin = window.location.origin;

    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
        const url =
            typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
        const method = init?.method ?? (input instanceof Request ? input.method : 'GET');
        const response = await originalFetch(input, init);
        if (response.ok && isDataMutation(method, url, origin)) bump();
        return response;
    };
});
