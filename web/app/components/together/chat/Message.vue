<script setup lang="ts">
import type { ChatMessage } from '~/types/together';
import type { ReactionLimits, ReactionPill } from '~/utils/reactions';
import type ChatReactions from '~/components/chat/Reactions.vue';

// One chat line: text, links and :emote: tokens (ChatRichText: text nodes, images and
// anchors, never HTML), and its reactions (pills, "Add reaction" on hover or long press).
const props = defineProps<{
    message: ChatMessage;
    canDelete: boolean;
    mine: boolean;
    pills: ReactionPill[];
    // connected: may react (and remove my reactions)
    canReact: boolean;
    reactionLimits: ReactionLimits;
}>();

const emit = defineEmits<{
    delete: [id: number];
    react: [id: number, key: string, add: boolean];
}>();

const reactionsEl = ref<InstanceType<typeof ChatReactions> | null>(null);

// Anonymous author shown under the alias (a moderator gets the real name and a badge)
const anonymousAlias = computed(() => !!props.message.anonymous && props.message.user_id === null);
const time = computed(() =>
    new Date(props.message.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
);
const canAddNew = computed(
    () => props.canReact && canAddReaction(props.pills, props.reactionLimits),
);

const openPicker = (anchor: HTMLElement) => reactionsEl.value?.openPicker(anchor);
const longPress = useLongPress((el) => openPicker(el));
</script>

<template>
    <div
        v-if="message.system"
        class="text-dim flex items-center gap-1.5 px-2 py-0.5 text-[11px] italic"
    >
        <Icon name="heroicons:information-circle" size="11" class="text-lava/70 shrink-0" />
        <span class="min-w-0 break-words">{{ message.text }}</span>
    </div>
    <div
        v-else
        class="group relative rounded-md px-2 py-1 transition-colors hover:bg-white/[0.03]"
        v-on="longPress"
    >
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
        <div class="min-w-0 text-xs leading-relaxed break-words text-white/85">
            <ChatRichText :text="message.text" />
        </div>
        <ChatReactions
            ref="reactionsEl"
            :pills="pills"
            :can-react="canReact"
            :can-remove="canReact"
            :limits="reactionLimits"
            @toggle="(key, add) => emit('react', message.id, key, add)"
        />
        <div
            v-if="canAddNew || canDelete"
            class="bg-panel/90 border-border absolute top-0.5 right-1 hidden items-center rounded-md
                border group-hover:flex"
        >
            <button
                v-if="canAddNew"
                type="button"
                class="text-dim hover:text-lava flex h-5 w-5 items-center justify-center rounded
                    transition-colors"
                title="Add reaction"
                @click="openPicker($event.currentTarget as HTMLElement)"
            >
                <Icon name="heroicons:face-smile" size="11" />
            </button>
            <button
                v-if="canDelete"
                type="button"
                class="text-dim flex h-5 w-5 items-center justify-center rounded transition-colors
                    hover:text-red-400"
                title="Delete message"
                @click="emit('delete', message.id)"
            >
                <Icon name="heroicons:trash" size="11" />
            </button>
        </div>
    </div>
</template>
