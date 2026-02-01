/**
 * Composable for scene star rating with hover states.
 * Supports half-star ratings (0.5 increments).
 */
export const useSceneRating = (sceneId: Ref<number | undefined>) => {
    const { setSceneRating, deleteSceneRating } = useApiScenes();

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
        if (!sceneId.value) return;
        const newRating = isLeftHalf ? starIndex - 0.5 : starIndex;

        if (newRating === currentRating.value) {
            // Clicking same rating clears it
            const oldRating = currentRating.value;
            currentRating.value = 0;
            try {
                await deleteSceneRating(sceneId.value);
            } catch {
                // Revert on error
                currentRating.value = oldRating;
            }
        } else {
            const oldRating = currentRating.value;
            currentRating.value = newRating;
            try {
                await setSceneRating(sceneId.value, newRating);
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
