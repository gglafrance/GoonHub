import type { ShallowRef } from 'vue';
import type videojs from 'video.js';

type Player = ReturnType<typeof videojs>;

/**
 * Composable for handling scene resume position.
 */
export const useResumePosition = (
    player: ShallowRef<Player | null>,
    sceneId: Ref<number | undefined>,
) => {
    const { getResumePosition } = useApiScenes();

    const resumePosition = ref<number | null>(null);
    const showResumePrompt = ref(false);

    const loadResumePosition = async () => {
        if (!sceneId.value) return;

        try {
            const result = await getResumePosition(sceneId.value);
            if (result.position && result.position > 30) {
                // Only show resume if more than 30 seconds in
                resumePosition.value = result.position;
                showResumePrompt.value = true;
            }
        } catch {
            // Silently fail
        }
    };

    const resumePlayback = () => {
        if (player.value && resumePosition.value) {
            player.value.currentTime(resumePosition.value);
            player.value.play()?.catch(() => {
                // Autoplay may be blocked
            });
        }
        showResumePrompt.value = false;
    };

    const dismissResume = () => {
        showResumePrompt.value = false;
    };

    const formatResumeTime = (seconds: number): string => {
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    };

    return {
        resumePosition,
        showResumePrompt,
        loadResumePosition,
        resumePlayback,
        dismissResume,
        formatResumeTime,
    };
};
