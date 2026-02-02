import type { JobStatusData } from '~/types/jobs';

interface SceneEventData {
    type: string;
    scene_id: number;
    data?: Record<string, unknown>;
}

type FieldExtractor = (event: SceneEventData) => Record<string, unknown>;

// Events that update scene fields
const SCENE_UPDATE_HANDLERS: Record<string, FieldExtractor> = {
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

// Events that remove scenes from the store
const SCENE_REMOVE_EVENTS = ['scene:trashed', 'scene:deleted'];

// Events that restore scenes (trigger reload)
const SCENE_RESTORE_EVENTS = ['scene:restored'];

// Combine all event types for iteration
const EVENT_HANDLERS: Record<string, FieldExtractor> = SCENE_UPDATE_HANDLERS;

function handleSSEEvent(
    eventType: string,
    rawData: string,
    sceneStore: ReturnType<typeof useSceneStore>,
) {
    const event: SceneEventData = JSON.parse(rawData);

    // Handle scene removal events (trashed or deleted)
    if (SCENE_REMOVE_EVENTS.includes(eventType)) {
        sceneStore.removeScene(event.scene_id);
        return;
    }

    // Handle scene restore events (trigger a reload to get the scene back)
    if (SCENE_RESTORE_EVENTS.includes(eventType)) {
        sceneStore.loadScenes(sceneStore.currentPage);
        return;
    }

    // Handle scene update events
    const handler = EVENT_HANDLERS[eventType];
    if (!handler) return;

    sceneStore.updateSceneFields(event.scene_id, handler(event));
}

function supportsSharedWorker(): boolean {
    return (
        typeof SharedWorker !== 'undefined' &&
        typeof BroadcastChannel !== 'undefined' &&
        window.isSecureContext
    );
}

function supportsBroadcastChannel(): boolean {
    return typeof BroadcastChannel !== 'undefined';
}

function useSSESharedWorker() {
    const authStore = useAuthStore();
    const sceneStore = useSceneStore();
    const jobStatusStore = useJobStatusStore();

    let channel: BroadcastChannel | null = null;
    let worker: SharedWorker | null = null;
    let joined = false;

    function onChannelMessage(e: MessageEvent) {
        const { type, eventType, data } = e.data;

        if (type === 'worker-ready') {
            channel?.postMessage({ type: 'tab-join' });
            channel?.postMessage({ type: 'connect' });
        } else if (type === 'sse-event') {
            if (eventType === 'jobs:status') {
                const status: JobStatusData = JSON.parse(data);
                jobStatusStore.updateStatus(status);
                jobStatusStore.setConnected(true);
            } else {
                handleSSEEvent(eventType, data, sceneStore);
            }
        } else if (type === 'sse-connected') {
            jobStatusStore.setConnected(true);
        } else if (type === 'sse-reconnecting') {
            jobStatusStore.setConnected(false);
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

        const workerUrl = new URL('/_worker/sse', window.location.origin).href;

        try {
            worker = new SharedWorker(workerUrl, { name: 'sse-worker' });
            worker.onerror = () => {};

            channel = new BroadcastChannel('sse-events');
            channel.onmessage = onChannelMessage;
            joined = true;

            window.addEventListener('beforeunload', onBeforeUnload);
        } catch {
            // SharedWorker failed, fallback mode will be used on next page load
        }
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

function useSSELeaderElection() {
    const authStore = useAuthStore();
    const sceneStore = useSceneStore();
    const jobStatusStore = useJobStatusStore();

    const tabId = Math.random().toString(36).slice(2) + Date.now().toString(36);
    let channel: BroadcastChannel | null = null;
    let eventSource: EventSource | null = null;
    let role: 'init' | 'electing' | 'leader' | 'follower' = 'init';
    let electionTimer: ReturnType<typeof setTimeout> | null = null;
    let heartbeatInterval: ReturnType<typeof setInterval> | null = null;
    let heartbeatTimeout: ReturnType<typeof setTimeout> | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectDelay = 1000;
    const maxReconnectDelay = 30000;

    function dispatchEvent(eventType: string, data: string) {
        if (eventType === 'jobs:status') {
            const status: JobStatusData = JSON.parse(data);
            jobStatusStore.updateStatus(status);
            jobStatusStore.setConnected(true);
        } else {
            handleSSEEvent(eventType, data, sceneStore);
        }
    }

    function openEventSource() {
        closeEventSource();

        const url = '/api/v1/events';
        eventSource = new EventSource(url, { withCredentials: true });

        eventSource.onopen = () => {
            reconnectDelay = 1000;
            jobStatusStore.setConnected(true);
            channel?.postMessage({ type: 'sse-connected' });
        };

        eventSource.addEventListener('jobs:status', (e: MessageEvent) => {
            dispatchEvent('jobs:status', e.data);
            channel?.postMessage({ type: 'sse-event', eventType: 'jobs:status', data: e.data });
        });

        for (const eventType of Object.keys(EVENT_HANDLERS)) {
            eventSource.addEventListener(eventType, (e: MessageEvent) => {
                dispatchEvent(eventType, e.data);
                channel?.postMessage({ type: 'sse-event', eventType, data: e.data });
            });
        }

        eventSource.onerror = () => {
            eventSource?.close();
            eventSource = null;
            jobStatusStore.setConnected(false);
            channel?.postMessage({ type: 'sse-reconnecting' });
            scheduleReconnect();
        };
    }

    function closeEventSource() {
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
        if (!authStore.isAuthenticated || role !== 'leader') return;

        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            sceneStore.loadScenes(sceneStore.currentPage);
            if (role === 'leader') openEventSource();
        }, reconnectDelay);

        reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
    }

    function promoteToLeader() {
        role = 'leader';
        startHeartbeat();
        openEventSource();
    }

    function demoteToFollower() {
        role = 'follower';
        stopHeartbeat();
        closeEventSource();
        resetHeartbeatTimeout();
    }

    function startHeartbeat() {
        stopHeartbeat();
        channel?.postMessage({ type: 'leader-heartbeat', tabId });
        heartbeatInterval = setInterval(() => {
            channel?.postMessage({ type: 'leader-heartbeat', tabId });
        }, 5000);
    }

    function stopHeartbeat() {
        if (heartbeatInterval) {
            clearInterval(heartbeatInterval);
            heartbeatInterval = null;
        }
    }

    function resetHeartbeatTimeout() {
        if (heartbeatTimeout) clearTimeout(heartbeatTimeout);
        heartbeatTimeout = setTimeout(() => {
            heartbeatTimeout = null;
            if (role === 'follower') startElection();
        }, 10000);
    }

    function clearElectionTimer() {
        if (electionTimer) {
            clearTimeout(electionTimer);
            electionTimer = null;
        }
    }

    function startElection() {
        role = 'electing';
        clearElectionTimer();

        channel?.postMessage({ type: 'announce', tabId });

        const jitter = 200 + Math.random() * 300;
        electionTimer = setTimeout(() => {
            if (role !== 'electing') return;
            channel?.postMessage({ type: 'claim-leader', tabId });

            electionTimer = setTimeout(() => {
                if (role !== 'electing') return;
                promoteToLeader();
            }, 200);
        }, jitter);
    }

    function onChannelMessage(e: MessageEvent) {
        const msg = e.data;

        switch (msg.type) {
            case 'leader-heartbeat':
                if (role === 'electing') {
                    clearElectionTimer();
                    demoteToFollower();
                } else if (role === 'follower') {
                    resetHeartbeatTimeout();
                } else if (role === 'leader' && msg.tabId !== tabId) {
                    if (tabId > msg.tabId) {
                        demoteToFollower();
                    }
                }
                break;

            case 'claim-leader':
                if (role === 'electing' && msg.tabId < tabId) {
                    clearElectionTimer();
                    demoteToFollower();
                }
                break;

            case 'announce':
                if (role === 'leader') {
                    channel?.postMessage({ type: 'leader-heartbeat', tabId });
                }
                break;

            case 'leader-leaving':
                if (role === 'follower') {
                    if (heartbeatTimeout) clearTimeout(heartbeatTimeout);
                    heartbeatTimeout = null;
                    startElection();
                }
                break;

            case 'sse-event':
                if (role === 'follower') {
                    dispatchEvent(msg.eventType, msg.data);
                }
                break;

            case 'sse-connected':
                if (role === 'follower') {
                    jobStatusStore.setConnected(true);
                }
                break;

            case 'sse-reconnecting':
                if (role === 'follower') {
                    jobStatusStore.setConnected(false);
                    sceneStore.loadScenes(sceneStore.currentPage);
                }
                break;

            case 'disconnect-all':
                disconnect();
                break;
        }
    }

    function onBeforeUnload() {
        if (role === 'leader') {
            channel?.postMessage({ type: 'leader-leaving', tabId });
        }
    }

    function connect() {
        if (!authStore.isAuthenticated) return;
        disconnect();

        channel = new BroadcastChannel('sse-leader');
        channel.onmessage = onChannelMessage;

        window.addEventListener('beforeunload', onBeforeUnload);

        startElection();
    }

    function disconnect() {
        window.removeEventListener('beforeunload', onBeforeUnload);

        clearElectionTimer();
        stopHeartbeat();
        if (heartbeatTimeout) {
            clearTimeout(heartbeatTimeout);
            heartbeatTimeout = null;
        }
        closeEventSource();

        if (channel) {
            channel.onmessage = null;
            channel.close();
            channel = null;
        }

        role = 'init';
    }

    function disconnectAll() {
        channel?.postMessage({ type: 'disconnect-all' });
        disconnect();
    }

    return { connect, disconnect: disconnectAll };
}

function useSSEFallback() {
    const authStore = useAuthStore();
    const sceneStore = useSceneStore();
    const jobStatusStore = useJobStatusStore();

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
            jobStatusStore.setConnected(true);
        };

        eventSource.addEventListener('jobs:status', (e: MessageEvent) => {
            const status: JobStatusData = JSON.parse(e.data);
            jobStatusStore.updateStatus(status);
            jobStatusStore.setConnected(true);
        });

        // Scene update handlers
        for (const [eventType, handler] of Object.entries(EVENT_HANDLERS)) {
            eventSource.addEventListener(eventType, (e: MessageEvent) => {
                const event: SceneEventData = JSON.parse(e.data);
                sceneStore.updateSceneFields(event.scene_id, handler(event));
            });
        }

        // Scene remove handlers (trash, delete)
        for (const eventType of SCENE_REMOVE_EVENTS) {
            eventSource.addEventListener(eventType, (e: MessageEvent) => {
                const event: SceneEventData = JSON.parse(e.data);
                sceneStore.removeScene(event.scene_id);
            });
        }

        // Scene restore handlers
        for (const eventType of SCENE_RESTORE_EVENTS) {
            eventSource.addEventListener(eventType, () => {
                sceneStore.loadScenes(sceneStore.currentPage);
            });
        }

        eventSource.onerror = () => {
            eventSource?.close();
            eventSource = null;
            jobStatusStore.setConnected(false);
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
    if (import.meta.client && supportsBroadcastChannel()) {
        return useSSELeaderElection();
    }
    return useSSEFallback();
};
