<script setup lang="ts">
// One-click reactions under the player, plus the full picker (emoji and server emotes).
const { socket } = useInjectedTogetherRoom();
const showPicker = ref(false);
const root = ref<HTMLElement | null>(null);

const react = (emote: string) => {
    socket.send({ t: 'reaction', emote });
};

const pick = (value: string) => {
    react(value);
    showPicker.value = false;
};

const onDocumentClick = (e: MouseEvent) => {
    if (showPicker.value && root.value && !root.value.contains(e.target as Node)) {
        showPicker.value = false;
    }
};
onMounted(() => document.addEventListener('click', onDocumentClick));
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick));
</script>

<template>
    <div ref="root" class="relative flex flex-wrap items-center gap-1">
        <span class="text-dim mr-1 text-[10px] font-medium tracking-wider uppercase">React</span>
        <button
            v-for="e in QUICK_REACTIONS"
            :key="e"
            class="border-border bg-panel hover:border-lava/30 flex h-7 w-7 items-center
                justify-center rounded-md border text-base transition-all hover:-translate-y-px"
            :title="`React with ${e}`"
            @click="react(e)"
        >
            {{ e }}
        </button>
        <button
            class="border-border bg-panel text-dim hover:border-lava/30 hover:text-lava flex h-7 w-7
                items-center justify-center rounded-md border transition-all"
            title="More reactions"
            @click.stop="showPicker = !showPicker"
        >
            <Icon name="heroicons:face-smile" size="14" />
        </button>
        <div v-if="showPicker" class="absolute top-full left-0 z-30 mt-2">
            <TogetherChatEmojiPicker @pick="pick" @close="showPicker = false" />
        </div>
    </div>
</template>
