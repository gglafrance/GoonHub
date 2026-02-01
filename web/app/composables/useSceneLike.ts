/**
 * Composable for scene like toggle with animation state.
 */
export const useSceneLike = (sceneId: Ref<number | undefined>) => {
    const { toggleSceneLike } = useApiScenes();

    const liked = ref(false);
    const animating = ref(false);

    const toggle = async () => {
        if (!sceneId.value) return;
        const wasLiked = liked.value;
        liked.value = !wasLiked;
        animating.value = true;
        setTimeout(() => {
            animating.value = false;
        }, 300);

        try {
            const res = await toggleSceneLike(sceneId.value);
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
