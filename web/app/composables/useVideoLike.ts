/**
 * Composable for video like toggle with animation state.
 */
export const useVideoLike = (videoId: Ref<number | undefined>) => {
    const { toggleVideoLike } = useApiVideos();

    const liked = ref(false);
    const animating = ref(false);

    const toggle = async () => {
        if (!videoId.value) return;
        const wasLiked = liked.value;
        liked.value = !wasLiked;
        animating.value = true;
        setTimeout(() => {
            animating.value = false;
        }, 300);

        try {
            const res = await toggleVideoLike(videoId.value);
            liked.value = res.liked;
        } catch {
            liked.value = wasLiked;
        }
    };

    const setLiked = (value: boolean) => {
        liked.value = value;
    };

    return {
        liked,
        animating,
        toggle,
        setLiked,
    };
};
