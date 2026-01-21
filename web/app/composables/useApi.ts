export const useApi = () => {
    const uploadVideo = async (file: File, title?: string) => {
        const formData = new FormData();
        formData.append('video', file);
        if (title) {
            formData.append('title', title);
        }

        const response = await fetch('/api/v1/videos', {
            method: 'POST',
            body: formData,
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to upload video');
        }

        return await response.json();
    };

    const fetchVideos = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const response = await fetch(`/api/v1/videos?${params}`);

        if (!response.ok) {
            throw new Error('Failed to fetch videos');
        }

        return await response.json();
    };

    return {
        uploadVideo,
        fetchVideos,
    };
};
