/**
 * Video-related API operations: CRUD, search, streaming, filters, interactions.
 */
export const useApiVideos = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const uploadVideo = async (file: File, title?: string) => {
        const formData = new FormData();
        formData.append('video', file);
        if (title) {
            formData.append('title', title);
        }

        const response = await fetch('/api/v1/videos', {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchVideos = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const searchVideos = async (searchParams: Record<string, string | number | undefined>) => {
        const params = new URLSearchParams();
        for (const [key, value] of Object.entries(searchParams)) {
            if (value !== undefined && value !== '' && value !== 0) {
                params.set(key, String(value));
            }
        }

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchFilterOptions = async () => {
        const response = await fetch('/api/v1/videos/filters', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideo = async (id: number) => {
        const response = await fetch(`/api/v1/videos/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const updateVideoDetails = async (
        videoId: number,
        title: string,
        description: string,
        releaseDate?: string | null,
    ) => {
        const payload: Record<string, unknown> = { title, description };
        if (releaseDate !== undefined) {
            payload.release_date = releaseDate;
        }
        const response = await fetch(`/api/v1/videos/${videoId}/details`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(payload),
        });
        return handleResponse(response);
    };

    const extractThumbnail = async (videoId: number, timecode: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ timecode }),
        });
        return handleResponse(response);
    };

    const uploadThumbnail = async (videoId: number, file: File) => {
        const formData = new FormData();
        formData.append('thumbnail', file);

        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail/upload`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Video interactions
    const fetchVideoInteractions = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/interactions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideoRating = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setVideoRating = async (videoId: number, rating: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ rating }),
        });
        return handleResponse(response);
    };

    const deleteVideoRating = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const fetchVideoLike = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/like`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const toggleVideoLike = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchJizzedCount = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/jizzed`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const incrementJizzed = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/jizzed`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Watch tracking
    const recordWatch = async (
        videoId: number,
        duration: number,
        position: number,
        completed: boolean,
    ) => {
        const response = await fetch(`/api/v1/videos/${videoId}/watch`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ duration, position, completed }),
        });
        return handleResponse(response);
    };

    const getResumePosition = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/resume`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getVideoWatchHistory = async (videoId: number, limit = 10) => {
        const params = new URLSearchParams({ limit: limit.toString() });
        const response = await fetch(`/api/v1/videos/${videoId}/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getUserWatchHistory = async (page = 1, limit = 20) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRelatedVideos = async (videoId: number, limit = 12) => {
        const params = new URLSearchParams({ limit: limit.toString() });
        const response = await fetch(`/api/v1/videos/${videoId}/related?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        uploadVideo,
        fetchVideos,
        searchVideos,
        fetchFilterOptions,
        fetchVideo,
        updateVideoDetails,
        extractThumbnail,
        uploadThumbnail,
        fetchVideoInteractions,
        fetchVideoRating,
        setVideoRating,
        deleteVideoRating,
        fetchVideoLike,
        toggleVideoLike,
        fetchJizzedCount,
        incrementJizzed,
        recordWatch,
        getResumePosition,
        getVideoWatchHistory,
        getUserWatchHistory,
        fetchRelatedVideos,
    };
};
