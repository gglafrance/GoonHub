interface VideoEventData {
    type: string;
    video_id: number;
    data?: Record<string, any>;
}

export const useSSE = () => {
    const authStore = useAuthStore();
    const videoStore = useVideoStore();

    let eventSource: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectDelay = 1000;
    const maxReconnectDelay = 30000;

    function connect() {
        if (!authStore.isAuthenticated) return;
        disconnect();

        // Use credentials to send HTTP-only cookies for authentication
        // No longer passing token in URL to prevent exposure in logs/history
        const url = '/api/v1/events';
        eventSource = new EventSource(url, { withCredentials: true });

        eventSource.onopen = () => {
            reconnectDelay = 1000;
        };

        eventSource.addEventListener('video:metadata_complete', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                duration: event.data?.duration,
                width: event.data?.width,
                height: event.data?.height,
                processing_status: 'processing',
            });
        });

        eventSource.addEventListener('video:thumbnail_complete', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                thumbnail_path: event.data?.thumbnail_path,
            });
        });

        eventSource.addEventListener('video:sprites_complete', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                vtt_path: event.data?.vtt_path,
                sprite_sheet_path: event.data?.sprite_sheet_path,
            });
        });

        eventSource.addEventListener('video:completed', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                processing_status: 'completed',
            });
        });

        eventSource.addEventListener('video:failed', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                processing_status: 'failed',
                processing_error: event.data?.error,
            });
        });

        eventSource.addEventListener('video:cancelled', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                processing_status: 'cancelled',
            });
        });

        eventSource.addEventListener('video:timed_out', (e: MessageEvent) => {
            const event: VideoEventData = JSON.parse(e.data);
            videoStore.updateVideoFields(event.video_id, {
                processing_status: 'timed_out',
            });
        });

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
};
