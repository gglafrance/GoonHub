import type { Video } from '~/types/video';

export const useFramePreloader = (video: Video) => {
    const frames = ref<HTMLImageElement[]>([]);
    const loadingErrors = ref<Set<number>>(new Set());
    const isLoaded = ref(false);
    const isLoading = ref(false);

    const loadFrames = async () => {
        if (!video.frame_paths || isLoaded.value || isLoading.value) return;

        isLoading.value = true;
        const framePaths = video.frame_paths.split(',');
        const frameImages: HTMLImageElement[] = [];
        const errors = new Set<number>();

        for (let i = 0; i < framePaths.length; i++) {
            const framePath = framePaths[i];
            const img = new Image();

            try {
                await new Promise<void>((resolve, reject) => {
                    img.onload = () => resolve();
                    img.onerror = () => reject(new Error(`Failed to load frame ${framePath}`));
                    img.src = `/frames/${video.id}/${framePath}`;
                });
                frameImages[i] = img;
            } catch (err) {
                errors.add(i);
            }
        }

        frames.value = frameImages;
        loadingErrors.value = errors;
        isLoaded.value = true;
        isLoading.value = false;
    };

    const getFrameByTimestamp = (timestamp: number): HTMLImageElement | null => {
        if (!video.frame_interval || frames.value.length === 0) return null;

        const index = Math.min(
            Math.floor(timestamp / video.frame_interval),
            frames.value.length - 1,
        );

        return frames.value[index] || null;
    };

    const hasError = (index: number): boolean => {
        return loadingErrors.value.has(index);
    };

    return {
        frames,
        isLoaded,
        isLoading,
        loadFrames,
        getFrameByTimestamp,
        hasError,
    };
};
