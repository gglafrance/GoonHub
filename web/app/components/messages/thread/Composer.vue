<script setup lang="ts">
import type { SendMessageBody } from '~/types/messages';

// Sticky input of a thread: text with emoji and server emotes (TogetherChatInput), or why the
// thread is read-only. Videos, markers, playlists and lobbies are shared from where they are
// (Share buttons), and show up here as cards.
const props = defineProps<{
    canSend: boolean;
    readOnlyText: string;
    maxLength: number;
    sending: boolean;
    error: string;
    otherName: string;
}>();
const emit = defineEmits<{
    send: [body: SendMessageBody];
    typing: [];
    dismissError: [];
}>();

const send = (text: string) => emit('send', { body: text });
</script>

<template>
    <div class="border-border border-t p-2.5">
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava mb-2 flex items-center gap-2 rounded-lg border
                px-2.5 py-1.5 text-[11px]"
        >
            <span class="flex-1">{{ error }}</span>
            <button class="hover:text-white" title="Dismiss" @click="emit('dismissError')">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <TogetherChatInput
            v-if="props.canSend"
            :max-length="props.maxLength"
            :disabled="props.sending"
            :placeholder="`Message ${props.otherName} (: for emotes)`"
            @send="send"
            @typing="emit('typing')"
        />
        <div
            v-else
            class="border-border bg-surface/40 text-dim flex items-center gap-2 rounded-lg border
                px-3 py-2 text-[11px]"
        >
            <Icon name="heroicons:lock-closed" size="13" class="shrink-0" />
            {{ props.readOnlyText || 'You cannot write in this conversation.' }}
        </div>
    </div>
</template>
