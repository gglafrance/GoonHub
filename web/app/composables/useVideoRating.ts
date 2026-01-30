/**
 * Composable for video star rating with hover states.
 * Supports half-star ratings (0.5 increments).
 */
export const useVideoRating = (videoId: Ref<number | undefined>) => {
    const { setVideoRating, deleteVideoRating } = useApiVideos();

    const currentRating = ref(0);
    const hoverRating = ref(0);
    const isHovering = ref(false);

    const displayRating = computed(() => (isHovering.value ? hoverRating.value : currentRating.value));

    const getStarState = (starIndex: number): 'full' | 'half' | 'empty' => {
        const rating = displayRating.value;
        if (rating >= starIndex) return 'full';
        if (rating >= starIndex - 0.5) return 'half';
        return 'empty';
    };

    const onStarHover = (starIndex: number, isLeftHalf: boolean) => {
        isHovering.value = true;
        hoverRating.value = isLeftHalf ? starIndex - 0.5 : starIndex;
    };

    const onStarLeave = () => {
        isHovering.value = false;
        hoverRating.value = 0;
    };

    const onStarClick = async (starIndex: number, isLeftHalf: boolean) => {
        if (!videoId.value) return;
        const newRating = isLeftHalf ? starIndex - 0.5 : starIndex;

        if (newRating === currentRating.value) {
            // Clicking same rating clears it
            const oldRating = currentRating.value;
            currentRating.value = 0;
            try {
                await deleteVideoRating(videoId.value);
            } catch {
                // Revert on error
                currentRating.value = oldRating;
            }
        } else {
            const oldRating = currentRating.value;
            currentRating.value = newRating;
            try {
                await setVideoRating(videoId.value, newRating);
            } catch {
                currentRating.value = oldRating;
            }
        }
    };

    const setRating = (rating: number) => {
        currentRating.value = rating;
    };

    return {
        currentRating,
        hoverRating,
        isHovering,
        displayRating,
        getStarState,
        onStarHover,
        onStarLeave,
        onStarClick,
        setRating,
    };
};
