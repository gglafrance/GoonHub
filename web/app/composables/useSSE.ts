interface VideoEventData {
    type: string;
    video_id: number;
    data?: Record<string, any>;
}

type FieldExtractor = (event: VideoEventData) => Record<string, any>;

const EVENT_HANDLERS: Record<string, FieldExtractor> = {
    'video:metadata_complete': (e) => ({
        duration: e.data?.duration,
        width: e.data?.width,
        height: e.data?.height,
        processing_status: 'processing',
    }),
    'video:thumbnail_complete': (e) => ({
        thumbnail_path: e.data?.thumbnail_path,
    }),
    'video:sprites_complete': (e) => ({
        vtt_path: e.data?.vtt_path,
        sprite_sheet_path: e.data?.sprite_sheet_path,
    }),
    'video:completed': () => ({
        processing_status: 'completed',
    }),
    'video:failed': (e) => ({
        processing_status: 'failed',
        processing_error: e.data?.error,
    }),
    'video:cancelled': () => ({
        processing_status: 'cancelled',
    }),
    'video:timed_out': () => ({
        processing_status: 'timed_out',
    }),
};

function handleSSEEvent(
    eventType: string,
    rawData: string,
    videoStore: ReturnType<typeof useVideoStore>,
) {
    const handler = EVENT_HANDLERS[eventType];
    if (!handler) return;

    const event: VideoEventData = JSON.parse(rawData);
    videoStore.updateVideoFields(event.video_id, handler(event));
}

function supportsSharedWorker(): boolean {
    return typeof SharedWorker !== 'undefined' && typeof BroadcastChannel !== 'undefined';
}

function useSSESharedWorker() {
    const authStore = useAuthStore();
    const videoStore = useVideoStore();

    let channel: BroadcastChannel | null = null;
    let worker: SharedWorker | null = null;
    let joined = false;

    function onChannelMessage(e: MessageEvent) {
        const { type, eventType, data } = e.data;

        if (type === 'sse-event') {
            handleSSEEvent(eventType, data, videoStore);
        } else if (type === 'sse-reconnecting') {
            videoStore.loadVideos(videoStore.currentPage);
        }
    }

    function onBeforeUnload() {
        if (channel && joined) {
            channel.postMessage({ type: 'tab-leave' });
            joined = false;
        }
    }

    function connect() {
        if (!authStore.isAuthenticated) return;
        disconnect();

        worker = new SharedWorker('/sse-worker.js', { name: 'sse-worker' });
        channel = new BroadcastChannel('sse-events');
        channel.onmessage = onChannelMessage;

        channel.postMessage({ type: 'tab-join' });
        channel.postMessage({ type: 'connect' });
        joined = true;

        window.addEventListener('beforeunload', onBeforeUnload);
    }

    function disconnect() {
        window.removeEventListener('beforeunload', onBeforeUnload);

        if (channel) {
            if (joined) {
                channel.postMessage({ type: 'tab-leave' });
                joined = false;
            }
            channel.onmessage = null;
            channel.close();
            channel = null;
        }

        if (worker) {
            worker = null;
        }
    }

    function disconnectAll() {
        if (channel) {
            channel.postMessage({ type: 'disconnect' });
        }
        disconnect();
    }

    return { connect, disconnect: disconnectAll };
}

function useSSEFallback() {
    const authStore = useAuthStore();
    const videoStore = useVideoStore();

    let eventSource: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectDelay = 1000;
    const maxReconnectDelay = 30000;

    function connect() {
        if (!authStore.isAuthenticated) return;
        disconnect();

        const url = '/api/v1/events';
        eventSource = new EventSource(url, { withCredentials: true });

        eventSource.onopen = () => {
            reconnectDelay = 1000;
        };

        for (const [eventType, handler] of Object.entries(EVENT_HANDLERS)) {
            eventSource.addEventListener(eventType, (e: MessageEvent) => {
                const event: VideoEventData = JSON.parse(e.data);
                videoStore.updateVideoFields(event.video_id, handler(event));
            });
        }

        eventSource.onerror = () => {
            eventSource?.close();
            eventSource = null;
            scheduleReconnect();
        };
    }

    function disconnect() {
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }
        reconnectDelay = 1000;
    }

    function scheduleReconnect() {
        if (!authStore.isAuthenticated) return;

        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            videoStore.loadVideos(videoStore.currentPage);
            connect();
        }, reconnectDelay);

        reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
    }

    return { connect, disconnect };
}

export const useSSE = () => {
    if (import.meta.client && supportsSharedWorker()) {
        return useSSESharedWorker();
    }
    return useSSEFallback();
};
