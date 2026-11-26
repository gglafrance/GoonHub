import type { ChatReactionEvent, Message, MessageReaction } from '~/types/messages';
import type { ReactionLimits } from '~/utils/reactions';

/**
 * Reactions of an open thread (docs/MESSAGES.md, "Reactions"): toggles my reaction with an
 * optimistic update replaced by the server's answer (reverted on error), and applies the
 * chat:reaction events of the conversation.
 */
export const useMessageReactions = (
    messages: Ref<Message[]>,
    conversationId: () => number | null,
    onError: (message: string) => void,
) => {
    const api = useApiMessages();
    const authStore = useAuthStore();
    const messagesStore = useMessagesStore();

    const limits = computed<ReactionLimits>(() => ({
        maxKinds: messagesStore.settings?.max_reaction_kinds ?? 20,
        maxPerUser: messagesStore.settings?.max_reactions_per_user ?? 10,
    }));

    const set = (messageId: number, reactions: MessageReaction[]) => {
        messages.value = messages.value.map((m) => (m.id === messageId ? { ...m, reactions } : m));
    };

    async function react(messageId: number, key: string, add: boolean) {
        const convId = conversationId();
        const user = authStore.user;
        if (!convId || !user) return;
        const before = messages.value.find((m) => m.id === messageId)?.reactions ?? [];
        const me = { id: user.id, username: user.username, display_name: '' };
        set(messageId, toggleMessageReaction(before, key, me, add));
        try {
            const res = add
                ? await api.addReaction(convId, messageId, key)
                : await api.removeReaction(convId, messageId, key);
            set(messageId, res.reactions ?? []);
        } catch (e: unknown) {
            set(messageId, before);
            onError(e instanceof Error ? e.message : 'Failed to react');
        }
    }

    /** A chat:reaction event (ignored for another conversation). */
    function apply(event: ChatReactionEvent) {
        if (event.conversation_id !== conversationId()) return;
        set(event.message_id, event.reactions ?? []);
    }

    return { limits, react, apply };
};
