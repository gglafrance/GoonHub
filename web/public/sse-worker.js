// SharedWorker for SSE connection sharing across browser tabs.
// All tabs share a single EventSource connection via BroadcastChannel.
// This file must remain plain JS (not bundled by Vite).

const SSE_URL = '/api/v1/events';
const CHANNEL_NAME = 'sse-events';

let eventSource = null;
let tabCount = 0;
let reconnectTimer = null;
let reconnectDelay = 1000;
const maxReconnectDelay = 30000;

const channel = new BroadcastChannel(CHANNEL_NAME);

const SSE_EVENT_TYPES = [
    'video:metadata_complete',
    'video:thumbnail_complete',
    'video:sprites_complete',
    'video:completed',
    'video:failed',
    'video:cancelled',
    'video:timed_out',
];

function broadcast(type, payload) {
    channel.postMessage({ type, ...payload });
}

function connectSSE() {
    if (eventSource) return;

    eventSource = new EventSource(SSE_URL, { withCredentials: true });

    eventSource.onopen = () => {
        reconnectDelay = 1000;
    };

    for (const eventType of SSE_EVENT_TYPES) {
        eventSource.addEventListener(eventType, (e) => {
            broadcast('sse-event', { eventType, data: e.data });
        });
    }

    eventSource.onerror = () => {
        disconnectSSE();
        broadcast('sse-reconnecting', {});
        scheduleReconnect();
    };
}

function disconnectSSE() {
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
    if (tabCount <= 0) return;

    reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connectSSE();
    }, reconnectDelay);

    reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
}

// Handle messages from tabs via BroadcastChannel
channel.onmessage = (e) => {
    const { type } = e.data;

    switch (type) {
        case 'tab-join':
            tabCount++;
            break;
        case 'tab-leave':
            tabCount = Math.max(0, tabCount - 1);
            if (tabCount === 0) {
                disconnectSSE();
            }
            break;
        case 'connect':
            connectSSE();
            break;
        case 'disconnect':
            disconnectSSE();
            tabCount = 0;
            break;
    }
};

// SharedWorker connect handler for port-based communication (keepalive)
self.onconnect = () => {};
