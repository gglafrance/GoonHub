interface SceneEventData {
    type: string;
    scene_id: number;
    data?: Record<string, any>;
}

type FieldExtractor = (event: SceneEventData) => Record<string, any>;

const EVENT_HANDLERS: Record<string, FieldExtractor> = {
    'scene:metadata_complete': (e) => ({
        duration: e.data?.duration,
        width: e.data?.width,
        height: e.data?.height,
        processing_status: 'processing',
    }),
    'scene:thumbnail_complete': (e) => ({
        thumbnail_path: e.data?.thumbnail_path,
    }),
    'scene:sprites_complete': (e) => ({
        vtt_path: e.data?.vtt_path,
        sprite_sheet_path: e.data?.sprite_sheet_path,
    }),
    'scene:completed': () => ({
        processing_status: 'completed',
    }),
    'scene:failed': (e) => ({
        processing_status: 'failed',
        processing_error: e.data?.error,
    }),
    'scene:cancelled': () => ({
        processing_status: 'cancelled',
    }),
    'scene:timed_out': () => ({
        processing_status: 'timed_out',
    }),
};

function handleSSEEvent(
    eventType: string,
    rawData: string,
    sceneStore: ReturnType<typeof useSceneStore>,
) {
    const handler = EVENT_HANDLERS[eventType];
    if (!handler) return;

    const event: SceneEventData = JSON.parse(rawData);
    sceneStore.updateSceneFields(event.scene_id, handler(event));
}

function supportsSharedWorker(): boolean {
    return typeof SharedWorker !== 'undefined' && typeof BroadcastChannel !== 'undefined';
}

function useSSESharedWorker() {
    const authStore = useAuthStore();
    const sceneStore = useSceneStore();

    let channel: BroadcastChannel | null = null;
    let worker: SharedWorker | null = null;
    let joined = false;

    function onChannelMessage(e: MessageEvent) {
        const { type, eventType, data } = e.data;

        if (type === 'sse-event') {
            handleSSEEvent(eventType, data, sceneStore);
        } else if (type === 'sse-reconnecting') {
            sceneStore.loadScenes(sceneStore.currentPage);
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
    const sceneStore = useSceneStore();

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
                const event: SceneEventData = JSON.parse(e.data);
                sceneStore.updateSceneFields(event.scene_id, handler(event));
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
            sceneStore.loadScenes(sceneStore.currentPage);
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
