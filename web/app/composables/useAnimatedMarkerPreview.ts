/**
 * IntersectionObserver-based composable for memory-efficient animated marker thumbnail playback.
 * Videos only play when 30%+ visible, pause and reset when scrolled away.
 */
export function useAnimatedMarkerPreview() {
    const observer = ref<IntersectionObserver | null>(null);

    onMounted(() => {
        observer.value = new IntersectionObserver(
            (entries) => {
                for (const entry of entries) {
                    const video = entry.target as HTMLVideoElement;
                    if (entry.isIntersecting) {
                        video.play().catch(() => {});
                    } else {
                        video.pause();
                        video.currentTime = 0;
                    }
                }
            },
            { threshold: 0.3 },
        );
    });

    const observe = (el: HTMLVideoElement | null) => {
        if (el && observer.value) observer.value.observe(el);
    };

    onBeforeUnmount(() => {
        observer.value?.disconnect();
    });

    return { observe };
}
