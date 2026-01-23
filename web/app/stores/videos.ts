import { defineStore } from 'pinia';
import type { Video, VideoListResponse } from '~/types/video';

export const useVideoStore = defineStore('videos', () => {
    const videos = ref<Video[]>([]);
    const total = ref(0);
    const currentPage = ref(1);
    const limit = ref(20);
    const isLoading = ref(false);
    const error = ref<string | null>(null);

    const { fetchVideos: apiFetchVideos, uploadVideo: apiUploadVideo } = useApi();

    const loadVideos = async (page = 1) => {
        isLoading.value = true;
        error.value = null;
        try {
            const response: VideoListResponse = await apiFetchVideos(page, limit.value);
            videos.value = response.data;
            total.value = response.total;
            currentPage.value = response.page;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
        }
    };

    const uploadVideo = async (file: File, title?: string) => {
        isLoading.value = true;
        error.value = null;
        try {
            await apiUploadVideo(file, title);
            await loadVideos(1);
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
            throw e;
        } finally {
            isLoading.value = false;
        }
    };

    return {
        videos,
        total,
        currentPage,
        limit,
        isLoading,
        error,
        loadVideos,
        uploadVideo,
    };
});
