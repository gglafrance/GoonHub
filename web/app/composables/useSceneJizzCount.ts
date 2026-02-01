/**
 * Composable for scene jizz count with increment animation.
 */
export const useSceneJizzCount = (sceneId: Ref<number | undefined>) => {
    const { incrementJizzed } = useApiScenes();

    const count = ref(0);
    const animating = ref(false);

    const increment = async () => {
        if (!sceneId.value) return;
        const prevCount = count.value;
        count.value++;
        animating.value = true;
        setTimeout(() => {
            animating.value = false;
        }, 300);

        try {
            const res = await incrementJizzed(sceneId.value);
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
