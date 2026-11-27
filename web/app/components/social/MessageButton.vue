<script setup lang="ts">
import type { Relationship } from '~/types/social';

// "Message" toward a user: opens the thread with them (/messages/<username>). Shown when I may
// send messages and their message settings let me write to them.
const props = withDefaults(
    defineProps<{ username: string; relationship: Relationship; variant?: 'button' | 'icon' }>(),
    { variant: 'button' },
);

const perms = useMyPermissions();
const visible = computed(
    () => !props.relationship.self && !!props.relationship.can_message && perms.has('messages:use'),
);
</script>

<template>
    <NuxtLink
        v-if="visible && variant === 'icon'"
        :to="messagesPath(username)"
        class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 w-7 shrink-0
            items-center justify-center rounded-md border transition-all"
        :title="`Message ${username}`"
        :aria-label="`Message ${username}`"
    >
        <Icon name="heroicons:chat-bubble-oval-left-ellipsis" size="13" />
    </NuxtLink>
    <NuxtLink
        v-else-if="visible"
        :to="messagesPath(username)"
        class="border-border text-dim hover:border-lava/30 flex items-center gap-1 rounded-md border
            px-2 py-1 text-[11px] font-medium transition-all hover:text-white"
    >
        <Icon name="heroicons:chat-bubble-oval-left-ellipsis" size="12" />
        Message
    </NuxtLink>
</template>
