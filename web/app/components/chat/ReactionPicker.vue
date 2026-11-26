<script setup lang="ts">
// The emoji and emote picker of the chats as a popover anchored to an element, teleported to
// the body so that the scrolling message lists do not clip it. open(anchor) shows it next to
// the anchor (above, or below when there is no room); a pick closes it and emits the key.
const emit = defineEmits<{ pick: [key: string] }>();

// size of TogetherChatEmojiPicker (w-72), for the placement
const PICKER_WIDTH = 288;
const PICKER_HEIGHT = 330;
const MARGIN = 8;

const visible = ref(false);
const pos = ref({ top: 0, left: 0 });
const panel = ref<HTMLElement | null>(null);
let anchorEl: HTMLElement | null = null;

const place = () => {
    if (!anchorEl) return;
    const r = anchorEl.getBoundingClientRect();
    const maxLeft = window.innerWidth - PICKER_WIDTH - MARGIN;
    const left = Math.max(MARGIN, Math.min(r.right - PICKER_WIDTH, maxLeft));
    let top = r.top - PICKER_HEIGHT - 6;
    if (top < MARGIN) top = Math.min(r.bottom + 6, window.innerHeight - PICKER_HEIGHT - MARGIN);
    pos.value = { top: Math.max(MARGIN, top), left };
};

const inside = (target: EventTarget | null) => {
    const node = target as Node | null;
    return !!node && (!!panel.value?.contains(node) || !!anchorEl?.contains(node));
};
const onPointerDown = (e: PointerEvent) => {
    if (!inside(e.target)) close();
};
// the list under the anchor scrolled: the popover would float away from it
const onScroll = (e: Event) => {
    if (!panel.value?.contains(e.target as Node)) close();
};
const onKeydown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') close();
};

// The document listeners only live while the popover is open (one picker per message)
const listen = (on: boolean) => {
    const method = on ? 'addEventListener' : 'removeEventListener';
    document[method]('pointerdown', onPointerDown as EventListener, true);
    document[method]('scroll', onScroll, true);
    document[method]('keydown', onKeydown as EventListener);
    window[method]('resize', close);
};

function close() {
    if (visible.value) listen(false);
    visible.value = false;
    anchorEl = null;
}

const open = (anchor: HTMLElement) => {
    if (!visible.value) listen(true);
    anchorEl = anchor;
    place();
    visible.value = true;
};

const toggle = (anchor: HTMLElement) => {
    if (visible.value && anchorEl === anchor) close();
    else open(anchor);
};

const onPick = (key: string) => {
    close();
    emit('pick', key);
};

onBeforeUnmount(close);

defineExpose({ open, close, toggle });
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            ref="panel"
            class="fixed z-[70]"
            :style="{ top: `${pos.top}px`, left: `${pos.left}px` }"
        >
            <TogetherChatEmojiPicker @pick="onPick" @close="close" />
        </div>
    </Teleport>
</template>
