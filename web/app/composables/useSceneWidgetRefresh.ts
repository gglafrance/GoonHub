import type { HomepageSectionData } from '~/types/homepage';

/**
 * Busy state of a homepage widget refresh: request() emits the refresh and stays busy until
 * the section data changes (or a timeout).
 */
export function useSceneWidgetRefresh(data: () => HomepageSectionData, emitRefresh: () => void) {
    const busy = ref(false);
    let timeout: ReturnType<typeof setTimeout> | null = null;

    const done = () => {
        busy.value = false;
        if (timeout) clearTimeout(timeout);
        timeout = null;
    };

    watch(data, done);

    const request = () => {
        if (busy.value) return;
        busy.value = true;
        timeout = setTimeout(done, 8000);
        emitRefresh();
    };

    onBeforeUnmount(done);

    return { busy, request };
}
