<script setup lang="ts">
import type { Conversation } from '~/types/messages';

// One conversation of a list (page or header popover): avatar with presence, name, last
// message, unread count, relative time, muted.
const props = defineProps<{ conversation: Conversation; active?: boolean; compact?: boolean }>();
const emit = defineEmits<{ navigate: [] }>();

const authStore = useAuthStore();
const messagesStore = useMessagesStore();
const title = computed(() => conversationTitle(props.conversation));
const to = computed(() =>
    props.conversation.other ? messagesPath(props.conversation.other.username) : '/messages',
);
const typing = computed(() => messagesStore.isTyping(props.conversation.id));
const preview = computed(() =>
    typing.value
        ? 'typing...'
        : messagePreviewText(props.conversation.last_message, authStore.user?.id ?? 0),
);
const unread = computed(() => unreadBadge(props.conversation.unread_count));
</script>

<template>
    <NuxtLink
        :to="to"
        class="group flex items-center gap-2.5 rounded-lg px-2 transition-colors"
        :class="[compact ? 'py-1.5' : 'py-2', active ? 'bg-lava/10' : 'hover:bg-white/[0.04]']"
        @click="emit('navigate')"
    >
        <div class="relative shrink-0">
            <SocialUserAvatar
                v-if="conversation.other"
                :username="conversation.other.username"
                :display-name="conversation.other.display_name"
                :user-id="conversation.other.id"
                :avatar-version="conversation.other.avatar_version"
                size="sm"
            />
            <div
                v-else
                class="border-border bg-surface text-dim flex h-8 w-8 items-center justify-center
                    rounded-full border"
            >
                <Icon name="heroicons:user" size="14" />
            </div>
            <SocialPresenceDot v-if="conversation.other" :user-id="conversation.other.id" />
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5">
                <span
                    class="truncate text-xs"
                    :class="
                        conversation.unread_count > 0
                            ? 'font-semibold text-white'
                            : 'font-medium text-white/85'
                    "
                >
                    {{ title }}
                </span>
                <Icon
                    v-if="conversation.muted"
                    name="heroicons:bell-slash"
                    size="11"
                    class="text-dim shrink-0"
                    title="Muted"
                />
                <span class="text-dim ml-auto shrink-0 font-mono text-[9px]">
                    <NuxtTime :datetime="conversation.updated_at" relative />
                </span>
            </div>
            <div class="flex items-center gap-1.5">
                <span
                    class="min-w-0 flex-1 truncate text-[11px]"
                    :class="[
                        typing ? 'text-lava italic' : '',
                        !typing && conversation.unread_count > 0 ? 'text-white/80' : 'text-dim',
                    ]"
                >
                    {{ preview }}
                </span>
                <span
                    v-if="unread"
                    class="flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full px-1
                        font-mono text-[9px] font-semibold text-white"
                    :class="conversation.muted ? 'bg-white/20' : 'bg-lava'"
                >
                    {{ unread }}
                </span>
            </div>
        </div>
    </NuxtLink>
</template>
