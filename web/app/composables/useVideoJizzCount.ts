/**
 * Composable for video jizz count with increment animation.
 */
export const useVideoJizzCount = (videoId: Ref<number | undefined>) => {
    const { incrementJizzed } = useApiVideos();

    const count = ref(0);
    const animating = ref(false);

    const increment = async () => {
        if (!videoId.value) return;
        const prevCount = count.value;
        count.value++;
        animating.value = true;
        setTimeout(() => {
            animating.value = false;
        }, 300);

        try {
            const res = await incrementJizzed(videoId.value);
            count.value = res.count;
        } catch {
            count.value = prevCount;
        }
    };

    const setCount = (value: number) => {
        count.value = value;
    };

    return {
        count,
        animating,
        increment,
        setCount,
    };
};
