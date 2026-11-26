<script setup lang="ts">
// Shown when the browser refused to copy (plain http, no clipboard access): the text in a
// selected read-only field, to copy by hand. Place it in a `relative` parent.
const props = withDefaults(
    defineProps<{
        text: string;
        label?: string;
        align?: 'left' | 'right';
    }>(),
    { label: 'Copy it by hand', align: 'right' },
);

const emit = defineEmits<{
    close: [];
}>();

const input = ref<HTMLInputElement | null>(null);
const root = ref<HTMLElement | null>(null);

const select = () => {
    input.value?.focus();
    input.value?.select();
};

const onDocumentClick = (e: MouseEvent) => {
    if (root.value && !root.value.contains(e.target as Node)) emit('close');
};
const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') emit('close');
};

onMounted(() => {
    nextTick(select);
    // The click that opened it must not close it
    setTimeout(() => document.addEventListener('click', onDocumentClick), 0);
    document.addEventListener('keydown', onKey);
});
onBeforeUnmount(() => {
    document.removeEventListener('click', onDocumentClick);
    document.removeEventListener('keydown', onKey);
});
watch(
    () => props.text,
    () => nextTick(select),
);
</script>

<template>
    <div
        ref="root"
        class="border-border bg-panel/95 absolute top-full z-50 mt-2 w-80 max-w-[calc(100vw-2rem)]
            rounded-lg border p-2.5 shadow-2xl backdrop-blur-md"
        :class="align === 'right' ? 'right-0' : 'left-0'"
        role="dialog"
        :aria-label="label"
    >
        <div class="mb-1.5 flex items-center gap-1.5">
            <Icon name="heroicons:clipboard-document" size="12" class="text-lava" />
            <span class="text-[11px] font-medium text-white">{{ label }}</span>
            <button
                class="text-dim ml-auto flex h-5 w-5 items-center justify-center rounded
                    transition-colors hover:text-white"
                title="Close"
                @click="emit('close')"
            >
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <input
            ref="input"
            :value="text"
            readonly
            class="border-border bg-void/80 focus:border-lava/40 w-full rounded-md border px-2
                py-1.5 font-mono text-[11px] text-white outline-none"
            @focus="select"
            @click="select"
        />
        <p class="text-dim mt-1.5 text-[10px]">
            The browser blocked copying here. Press Ctrl+C (Cmd+C on a Mac).
        </p>
    </div>
</template>
