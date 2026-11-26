<script setup lang="ts">
import type { ReactionLimits, ReactionPill } from '~/utils/reactions';
import type ReactionPicker from '~/components/chat/ReactionPicker.vue';

// Reactions under a chat message (messages between users and the Goon2gether chat): one pill
// per key with its count, highlighted when I reacted; a click toggles my reaction, the title
// tells who reacted. Owns the picker: the trailing "+" and the message's "Add reaction" button
// (openPicker) open it. Not the floating reactions over the Goon2gether video.
const props = withDefaults(
    defineProps<{
        pills: ReactionPill[];
        // may add reactions (else only my own can be removed)
        canReact: boolean;
        // may remove my reactions (false while disconnected)
        canRemove?: boolean;
        limits: ReactionLimits;
        align?: 'start' | 'end';
    }>(),
    { canRemove: true, align: 'start' },
);

const emit = defineEmits<{
    toggle: [key: string, add: boolean];
}>();

const picker = ref<InstanceType<typeof ReactionPicker> | null>(null);

const canPickNew = computed(() => props.canReact && canAddReaction(props.pills, props.limits));

const enabled = (pill: ReactionPill) =>
    pill.mine
        ? props.canRemove
        : props.canReact && canAddReaction(props.pills, props.limits, pill.key);

const toggle = (key: string) => {
    const add = reactionAction(props.pills, key) === 'add';
    if (
        add ? !props.canReact || !canAddReaction(props.pills, props.limits, key) : !props.canRemove
    ) {
        return;
    }
    emit('toggle', key, add);
};

/** Opens the picker next to an element (the message's "Add reaction" button, a long press). */
const openPicker = (anchor: HTMLElement) => {
    if (canPickNew.value || props.pills.some((p) => enabled(p))) picker.value?.open(anchor);
};

defineExpose({ openPicker });
</script>

<template>
    <div>
        <div
            v-if="pills.length > 0"
            class="mt-1 flex flex-wrap items-center gap-1"
            :class="align === 'end' ? 'justify-end' : 'justify-start'"
        >
            <!-- not disabled when it cannot be clicked: the title (who reacted) still shows -->
            <button
                v-for="pill in pills"
                :key="pill.key"
                type="button"
                class="flex h-6 items-center gap-1 rounded-full border px-1.5 text-[11px]
                    font-medium transition-colors"
                :class="[
                    pill.mine
                        ? 'border-lava/50 bg-lava/15 text-white'
                        : 'border-border bg-white/[0.03] text-white/70',
                    enabled(pill)
                        ? pill.mine
                            ? 'hover:border-lava/80'
                            : 'hover:border-white/25 hover:text-white'
                        : 'cursor-default',
                ]"
                :title="reactionTitle(pill)"
                :aria-pressed="pill.mine"
                :aria-disabled="!enabled(pill)"
                :aria-label="reactionTitle(pill)"
                @click="toggle(pill.key)"
            >
                <ChatReactionEmote :value="pill.key" />
                <span class="font-mono tabular-nums">{{ pill.count }}</span>
            </button>
            <button
                v-if="canPickNew"
                type="button"
                class="border-border text-dim hover:text-lava hover:border-lava/40 flex h-6 w-7
                    items-center justify-center rounded-full border border-dashed transition-colors"
                title="Add reaction"
                @click="picker?.toggle($event.currentTarget as HTMLElement)"
            >
                <Icon name="heroicons:face-smile" size="12" />
            </button>
        </div>
        <ChatReactionPicker ref="picker" @pick="toggle" />
    </div>
</template>
