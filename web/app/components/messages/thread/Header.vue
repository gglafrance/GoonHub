<script setup lang="ts">
import type { Conversation, MessageUser } from '~/types/messages';

// Thread bar: back to the list (small screens), the other user (profile link, presence),
// mute.
const props = defineProps<{
    user: MessageUser | null;
    username: string;
    conversation: Conversation | null;
}>();

const api = useApiMessages();
const messagesStore = useMessagesStore();
const presenceStore = usePresenceStore();
const busy = ref(false);

const name = computed(() => props.user?.display_name || props.user?.username || props.username);
const status = computed(() => {
    if (!props.user) return '';
    const v = presenceStore.viewOf(props.user.id);
    if (!v) return '';
    const g = presenceGroupOf(v);
    return g === 'offline' ? '' : PRESENCE_GROUP_LABELS[g];
});

async function toggleMute() {
    const conv = props.conversation;
    if (!conv || busy.value) return;
    busy.value = true;
    try {
        messagesStore.upsert(await api.setMuted(conv.id, !conv.muted));
    } catch {
        // unchanged
    } finally {
        busy.value = false;
    }
}
</script>

<template>
    <div class="border-border flex items-center gap-2.5 border-b px-3 py-2">
        <NuxtLink
            to="/messages"
            class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 w-7
                items-center justify-center rounded-md border transition-all md:hidden"
            title="All conversations"
        >
            <Icon name="heroicons:arrow-left" size="13" />
        </NuxtLink>
        <NuxtLink
            v-if="user"
            :to="profilePath(user.username)"
            class="group flex min-w-0 items-center gap-2.5"
        >
            <div class="relative shrink-0">
                <SocialUserAvatar
                    :username="user.username"
                    :display-name="user.display_name"
                    :user-id="user.id"
                    :avatar-version="user.avatar_version"
                    size="sm"
                />
                <SocialPresenceDot :user-id="user.id" />
            </div>
            <div class="min-w-0">
                <div class="group-hover:text-lava truncate text-xs font-semibold text-white">
                    {{ name }}
                </div>
                <div class="text-dim truncate font-mono text-[10px]">
                    @{{ user.username }}<span v-if="status"> · {{ status }}</span>
                </div>
            </div>
        </NuxtLink>
        <span v-else class="text-sm font-semibold text-white">{{ username }}</span>
        <button
            v-if="conversation"
            class="border-border ml-auto flex h-7 w-7 items-center justify-center rounded-md border
                transition-all disabled:opacity-40"
            :class="
                conversation.muted
                    ? 'border-lava/30 text-lava'
                    : 'text-dim hover:border-lava/30 hover:text-lava'
            "
            :title="
                conversation.muted
                    ? 'Unmute (count it in the badge, show toasts)'
                    : 'Mute (no badge count nor toast)'
            "
            :disabled="busy"
            @click="toggleMute"
        >
            <Icon
                :name="conversation.muted ? 'heroicons:bell-slash' : 'heroicons:bell'"
                size="14"
            />
        </button>
    </div>
</template>
