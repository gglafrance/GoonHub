<script setup lang="ts">
import type { Emote } from '~/types/together';

// One emote: image, :name: (click to rename inline), size, animated badge, delete.
const props = defineProps<{
    emote: Emote;
}>();

const emit = defineEmits<{
    rename: [name: string];
    delete: [];
}>();

const editing = ref(false);
const draft = ref('');
const invalid = computed(() => !isValidEmoteName(draft.value));

const start = () => {
    draft.value = props.emote.name;
    editing.value = true;
};
const save = () => {
    if (invalid.value) return;
    editing.value = false;
    if (draft.value !== props.emote.name) emit('rename', draft.value);
};
</script>

<template>
    <div class="flex items-center gap-3 py-2">
        <div class="bg-void flex h-10 w-10 shrink-0 items-center justify-center rounded-md">
            <img :src="emote.url" :alt="emote.name" class="max-h-9 max-w-9 object-contain" />
        </div>
        <div class="min-w-0 flex-1">
            <div v-if="editing" class="flex items-center gap-1.5">
                <input
                    v-model="draft"
                    type="text"
                    maxlength="32"
                    class="border-border bg-surface w-44 rounded-md border px-2 py-1 font-mono
                        text-xs text-white outline-none"
                    :class="invalid ? 'border-lava/50' : 'focus:border-lava/40'"
                    @keydown.enter="save"
                    @keydown.esc="editing = false"
                />
                <button
                    class="text-emerald flex h-6 w-6 items-center justify-center rounded
                        disabled:opacity-40"
                    :disabled="invalid"
                    title="Save"
                    @click="save"
                >
                    <Icon name="heroicons:check" size="13" />
                </button>
                <button
                    class="text-dim flex h-6 w-6 items-center justify-center rounded
                        hover:text-white"
                    title="Cancel"
                    @click="editing = false"
                >
                    <Icon name="heroicons:x-mark" size="13" />
                </button>
            </div>
            <button
                v-else
                class="hover:text-lava font-mono text-xs text-white transition-colors"
                title="Rename"
                @click="start"
            >
                :{{ emote.name }}:
            </button>
            <div class="text-dim mt-0.5 flex items-center gap-2 text-[10px]">
                <span class="font-mono">{{ emote.width }}x{{ emote.height }}</span>
                <span
                    v-if="emote.animated"
                    class="bg-lava/15 text-lava rounded px-1 py-px text-[9px] font-semibold
                        tracking-wider uppercase"
                >
                    Animated
                </span>
            </div>
        </div>
        <button
            class="text-dim flex h-7 w-7 items-center justify-center rounded-md transition-colors
                hover:bg-red-500/10 hover:text-red-400"
            title="Delete"
            @click="emit('delete')"
        >
            <Icon name="heroicons:trash" size="13" />
        </button>
    </div>
</template>
