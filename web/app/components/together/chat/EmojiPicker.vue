<script setup lang="ts">
// Emoji and server emote picker (popover). Emits the emoji, or `:name:` for a server emote.
const emit = defineEmits<{
    pick: [value: string];
    close: [];
}>();

const { emotes, load } = useTogetherEmotes();
onMounted(() => load());

const tab = ref<string>('emotes');
const query = ref('');

const tabs = computed(() => [
    { id: 'emotes', label: 'Server emotes', icon: 'heroicons:star' },
    ...TOGETHER_EMOJI.map((c) => ({ id: c.id, label: c.label, icon: c.icon })),
]);

watch(
    () => emotes.value.length,
    (n) => {
        if (n === 0 && tab.value === 'emotes') tab.value = TOGETHER_EMOJI[0]!.id;
    },
    { immediate: true },
);

const filteredEmotes = computed(() =>
    query.value.trim() ? matchEmotes(emotes.value, query.value.trim(), 200) : emotes.value,
);
const emoji = computed(() => TOGETHER_EMOJI.find((c) => c.id === tab.value)?.emoji ?? []);

const onKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') emit('close');
};
</script>

<template>
    <div
        class="border-border bg-panel/95 w-72 rounded-xl border p-2 shadow-2xl backdrop-blur-md"
        @keydown="onKeydown"
    >
        <div class="mb-2 flex items-center gap-1">
            <button
                v-for="t in tabs"
                :key="t.id"
                class="flex h-7 w-7 items-center justify-center rounded-md transition-colors"
                :class="t.id === tab ? 'bg-lava/15 text-lava' : 'text-dim hover:text-white'"
                :title="t.label"
                @click="tab = t.id"
            >
                <Icon :name="t.icon" size="14" />
            </button>
        </div>

        <template v-if="tab === 'emotes'">
            <input
                v-model="query"
                type="text"
                placeholder="Search emotes"
                class="border-border bg-surface focus:border-lava/40 mb-2 w-full rounded-md border
                    px-2 py-1 text-xs text-white outline-none"
            />
            <div class="grid max-h-56 grid-cols-6 gap-1 overflow-y-auto">
                <button
                    v-for="e in filteredEmotes"
                    :key="e.id"
                    class="flex h-10 items-center justify-center rounded-md transition-colors
                        hover:bg-white/10"
                    :title="`:${e.name}:`"
                    @click="emit('pick', `:${e.name}:`)"
                >
                    <img :src="e.url" :alt="e.name" class="h-8 w-8 object-contain" />
                </button>
                <p
                    v-if="filteredEmotes.length === 0"
                    class="text-dim col-span-6 py-4 text-center text-[11px]"
                >
                    No server emote
                </p>
            </div>
        </template>
        <div v-else class="grid max-h-60 grid-cols-8 gap-0.5 overflow-y-auto">
            <button
                v-for="e in emoji"
                :key="e"
                class="flex h-8 items-center justify-center rounded-md text-lg transition-colors
                    hover:bg-white/10"
                @click="emit('pick', e)"
            >
                {{ e }}
            </button>
        </div>
    </div>
</template>
