'use strict';

var SSE_URL = '/api/v1/events';
var CHANNEL_NAME = 'sse-events';

var eventSource = null;
var tabCount = 0;
var reconnectTimer = null;
var reconnectDelay = 1000;
var maxReconnectDelay = 30000;
var channel = null;

var SSE_EVENT_TYPES = [
    'scene:metadata_complete',
    'scene:thumbnail_complete',
    'scene:sprites_complete',
    'scene:completed',
    'scene:failed',
    'scene:cancelled',
    'scene:timed_out',
    'jobs:status',
];

function broadcast(type, payload) {
    if (channel) {
        try {
            var msg = { type: type };
            if (payload) {
                for (var key in payload) {
                    msg[key] = payload[key];
                }
            }
            channel.postMessage(msg);
        } catch {
            /* broadcast errors are non-fatal */
        }
    }
}

function connectSSE() {
    if (eventSource) return;

    try {
        eventSource = new EventSource(SSE_URL, { withCredentials: true });

        eventSource.onopen = function () {
            reconnectDelay = 1000;
            broadcast('sse-connected', {});
        };

        for (var i = 0; i < SSE_EVENT_TYPES.length; i++) {
            (function (eventType) {
                eventSource.addEventListener(eventType, function (e) {
                    broadcast('sse-event', { eventType: eventType, data: e.data });
                });
            })(SSE_EVENT_TYPES[i]);
        }

        eventSource.onerror = function () {
            disconnectSSE();
            broadcast('sse-reconnecting', {});
            scheduleReconnect();
        };
    } catch {
        /* SSE connection errors are handled via reconnect */
    }
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

    reconnectTimer = setTimeout(function () {
        reconnectTimer = null;
        connectSSE();
    }, reconnectDelay);

    reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
}

function handleMessage(e) {
    var type = e.data.type;

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
}

try {
    channel = new BroadcastChannel(CHANNEL_NAME);
    channel.onmessage = handleMessage;
    broadcast('worker-ready', {});
} catch {
    /* BroadcastChannel not available */
}

self.onconnect = function (e) {
    var port = e.ports[0];
    if (port) {
        port.start();
    }
};
