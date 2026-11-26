<script setup lang="ts">
import type { ChatMessage } from '~/types/together';

// One chat line: text and :emote: tokens (text nodes and images, never HTML).
const props = defineProps<{
    message: ChatMessage;
    canDelete: boolean;
    mine: boolean;
}>();

const emit = defineEmits<{
    delete: [id: number];
}>();

const { byName } = useTogetherEmotes();
const tokens = computed(() => tokenizeChat(props.message.text, byName.value));
// A message made of emotes only shows them bigger
const emoteOnly = computed(
    () =>
        tokens.value.length > 0 &&
        tokens.value.every((t) => t.kind === 'emote' || t.text.trim() === ''),
);
// Anonymous author shown under the alias (a moderator gets the real name and a badge)
const anonymousAlias = computed(() => !!props.message.anonymous && props.message.user_id === null);
const time = computed(() =>
    new Date(props.message.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
);
</script>

<template>
    <div
        v-if="message.system"
        class="text-dim flex items-center gap-1.5 px-2 py-0.5 text-[11px] italic"
    >
        <Icon name="heroicons:information-circle" size="11" class="text-lava/70 shrink-0" />
        <span class="min-w-0 break-words">{{ message.text }}</span>
    </div>
    <div v-else class="group relative rounded-md px-2 py-1 transition-colors hover:bg-white/[0.03]">
        <div class="flex items-baseline gap-1.5">
            <span
                class="flex shrink-0 items-center gap-1 text-[11px] font-semibold"
                :class="[mine ? 'text-lava' : 'text-white', { italic: anonymousAlias }]"
            >
                <Icon
                    v-if="message.anonymous"
                    name="heroicons:eye-slash"
                    size="10"
                    class="text-dim"
                />
                {{ message.username }}
            </span>
            <span
                v-if="message.anonymous && !anonymousAlias"
                class="border-border text-dim shrink-0 rounded border px-1 py-px text-[9px]"
                title="Sent anonymously: only moderators see who it is"
            >
                Anonymous #{{ message.anon_number }}
            </span>
            <span class="text-dim shrink-0 font-mono text-[9px]">{{ time }}</span>
        </div>
        <div
            class="min-w-0 text-xs leading-relaxed break-words text-white/85"
            :class="{ 'py-0.5': emoteOnly }"
        >
            <template v-for="(t, i) in tokens" :key="i">
                <img
                    v-if="t.kind === 'emote'"
                    :src="t.url"
                    :alt="`:${t.name}:`"
                    :title="`:${t.name}:`"
                    class="inline-block object-contain align-middle"
                    :class="emoteOnly ? 'h-8 w-8' : 'h-5 w-5'"
                />
                <span v-else class="whitespace-pre-wrap">{{ t.text }}</span>
            </template>
        </div>
        <button
            v-if="canDelete"
            class="text-dim absolute top-1 right-1 hidden h-5 w-5 items-center justify-center
                rounded transition-colors group-hover:flex hover:text-red-400"
            title="Delete message"
            @click="emit('delete', message.id)"
        >
            <Icon name="heroicons:trash" size="11" />
        </button>
    </div>
</template>
