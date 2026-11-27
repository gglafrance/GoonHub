import type { Emote } from '~/types/together';

// Shared by every component of the tab (room chat, picker, settings): loaded once, reloaded
// when the server says the emotes changed (`emotes` message) or after a change in Settings.
const emotes = ref<Emote[]>([]);
const loaded = ref(false);
let inflight: Promise<void> | null = null;

/**
 * Server custom emotes (GET /api/v1/together/emotes) with a lookup by lower-case name.
 */
export const useTogetherEmotes = () => {
    const { listEmotes } = useApiTogether();

    const load = (force = false): Promise<void> => {
        if (loaded.value && !force) return Promise.resolve();
        if (inflight) return inflight;
        inflight = (async () => {
            try {
                const res = await listEmotes();
                emotes.value = res.emotes ?? [];
                loaded.value = true;
            } catch {
                // Emotes are optional: messages show the :name: text meanwhile
            } finally {
                inflight = null;
            }
        })();
        return inflight;
    };

    const byName = computed(() => emoteMap(emotes.value));

    return { emotes, loaded, byName, load };
};
