<script setup lang="ts">
import type { Emote } from '~/types/together';

// Chat input: `:` autocomplete of the server emotes (arrows, Tab/Enter), emoji/emote picker.
// Shared by the Goon2gether chat and the messages (placeholder, typing, initial text, Escape).
const props = defineProps<{
    maxLength: number;
    disabled?: boolean;
    placeholder?: string;
    // prefilled text (editing a message)
    initial?: string;
    autofocus?: boolean;
}>();

const emit = defineEmits<{
    send: [text: string];
    // the user typed (for a typing indicator)
    typing: [];
    // Escape with nothing to close (cancels an edit)
    cancel: [];
}>();

const { emotes, load } = useTogetherEmotes();
onMounted(() => load());

const text = ref(props.initial ?? '');
const inputEl = ref<HTMLInputElement | null>(null);
const root = ref<HTMLElement | null>(null);
const showPicker = ref(false);
const caret = ref(0);
const highlighted = ref(0);
// Escape hides the autocomplete until the next keystroke
const autocompleteOff = ref(false);

const query = computed(() => emoteQueryAt(text.value, caret.value));
const suggestions = computed<Emote[]>(() =>
    query.value && !autocompleteOff.value ? matchEmotes(emotes.value, query.value.query) : [],
);
watch(suggestions, () => {
    highlighted.value = 0;
});

const onInput = () => {
    autocompleteOff.value = false;
    syncCaret();
    if (text.value.trim()) emit('typing');
};

const syncCaret = () => {
    caret.value = inputEl.value?.selectionStart ?? text.value.length;
};

const setText = async (value: string, at: number) => {
    text.value = value;
    await nextTick();
    inputEl.value?.focus();
    inputEl.value?.setSelectionRange(at, at);
    caret.value = at;
};

const complete = (emote: Emote) => {
    const q = query.value;
    if (!q) return;
    const res = completeEmote(text.value, q.start, caret.value, emote.name);
    setText(res.text, res.caret);
};

const insert = (value: string) => {
    const at = inputEl.value?.selectionStart ?? text.value.length;
    const before = text.value.slice(0, at);
    const sep = before && !before.endsWith(' ') ? ' ' : '';
    const piece = `${sep}${value} `;
    setText(before + piece + text.value.slice(at), at + piece.length);
    showPicker.value = false;
};

const submit = () => {
    const value = text.value.trim();
    if (!value || props.disabled) return;
    emit('send', value.slice(0, props.maxLength));
    text.value = '';
    caret.value = 0;
};

const onKeydown = (e: KeyboardEvent) => {
    if (suggestions.value.length > 0) {
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            const n = suggestions.value.length;
            highlighted.value = (highlighted.value + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
            return;
        }
        if (e.key === 'Tab' || e.key === 'Enter') {
            e.preventDefault();
            const emote = suggestions.value[highlighted.value];
            if (emote) complete(emote);
            return;
        }
        if (e.key === 'Escape') {
            autocompleteOff.value = true;
            return;
        }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        submit();
        return;
    }
    if (e.key === 'Escape' && !showPicker.value) emit('cancel');
};

const onDocumentClick = (e: MouseEvent) => {
    if (showPicker.value && root.value && !root.value.contains(e.target as Node)) {
        showPicker.value = false;
    }
};
onMounted(() => {
    document.addEventListener('click', onDocumentClick);
    if (props.autofocus) inputEl.value?.focus();
});
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick));
</script>

<template>
    <div ref="root" class="relative">
        <!-- Autocomplete -->
        <div
            v-if="suggestions.length > 0"
            class="border-border bg-panel/95 absolute right-0 bottom-full left-0 z-30 mb-1
                overflow-hidden rounded-lg border shadow-2xl backdrop-blur-md"
        >
            <button
                v-for="(e, i) in suggestions"
                :key="e.id"
                class="flex w-full items-center gap-2 px-2 py-1 text-left text-xs transition-colors"
                :class="i === highlighted ? 'bg-lava/15 text-white' : 'text-muted hover:bg-white/5'"
                @mousedown.prevent="complete(e)"
            >
                <img :src="e.url" :alt="e.name" class="h-5 w-5 object-contain" />
                <span class="font-mono">:{{ e.name }}:</span>
            </button>
        </div>

        <div v-if="showPicker" class="absolute right-0 bottom-full z-30 mb-1">
            <TogetherChatEmojiPicker @pick="insert" @close="showPicker = false" />
        </div>

        <div
            class="border-border bg-surface focus-within:border-lava/40 flex items-center gap-1
                rounded-lg border px-2 transition-colors"
        >
            <input
                ref="inputEl"
                v-model="text"
                type="text"
                :maxlength="maxLength"
                :disabled="disabled"
                :placeholder="placeholder || 'Send a message (: for emotes)'"
                class="min-w-0 flex-1 bg-transparent py-2 text-xs text-white outline-none
                    placeholder:text-white/30"
                @keydown="onKeydown"
                @input="onInput"
                @click="syncCaret"
                @keyup="syncCaret"
            />
            <button
                class="text-dim hover:text-lava flex h-6 w-6 shrink-0 items-center justify-center
                    transition-colors"
                title="Emoji and emotes"
                @click.stop="showPicker = !showPicker"
            >
                <Icon name="heroicons:face-smile" size="15" />
            </button>
            <button
                class="text-dim hover:text-lava flex h-6 w-6 shrink-0 items-center justify-center
                    transition-colors disabled:opacity-40"
                :disabled="disabled || !text.trim()"
                title="Send"
                @click="submit"
            >
                <Icon name="heroicons:paper-airplane" size="14" />
            </button>
        </div>
    </div>
</template>
