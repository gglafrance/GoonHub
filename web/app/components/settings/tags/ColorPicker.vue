<script setup lang="ts">
// Color swatch of a tag: click opens a small popover with the preset tag colors and a hex input.
const props = defineProps<{
    color: string;
    disabled?: boolean;
}>();

const emit = defineEmits<{
    select: [color: string];
}>();

const open = ref(false);
const hexInput = ref('');
const root = ref<HTMLElement | null>(null);

const parsedHex = computed(() => parseHexColor(hexInput.value));

function toggle() {
    if (props.disabled) return;
    open.value = !open.value;
    if (open.value) hexInput.value = props.color;
}

function pick(color: string) {
    open.value = false;
    if (color.toUpperCase() !== props.color.toUpperCase()) emit('select', color);
}

function applyHex() {
    if (parsedHex.value) pick(parsedHex.value);
}

function onDocumentMouseDown(e: MouseEvent) {
    if (root.value && !root.value.contains(e.target as Node)) open.value = false;
}

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') open.value = false;
}

watch(open, (isOpen) => {
    if (isOpen) {
        document.addEventListener('mousedown', onDocumentMouseDown);
        document.addEventListener('keydown', onKeydown);
    } else {
        document.removeEventListener('mousedown', onDocumentMouseDown);
        document.removeEventListener('keydown', onKeydown);
    }
});

onBeforeUnmount(() => {
    document.removeEventListener('mousedown', onDocumentMouseDown);
    document.removeEventListener('keydown', onKeydown);
});
</script>

<template>
    <div ref="root" class="relative flex items-center">
        <button
            type="button"
            class="h-3.5 w-3.5 rounded-full ring-1 ring-white/10 transition-all hover:scale-125
                hover:ring-white/40 disabled:cursor-not-allowed disabled:opacity-50"
            :style="{ backgroundColor: color }"
            :disabled="disabled"
            title="Change color"
            @click="toggle"
        />
        <div
            v-if="open"
            class="border-border bg-panel/95 absolute top-6 left-0 z-30 w-56 rounded-lg border p-3
                shadow-xl backdrop-blur-md"
        >
            <div class="grid grid-cols-7 gap-1.5">
                <button
                    v-for="preset in TAG_COLOR_PRESETS"
                    :key="preset"
                    type="button"
                    class="h-5 w-5 rounded-full border-2 transition-all"
                    :class="
                        preset.toUpperCase() === color.toUpperCase()
                            ? 'scale-110 border-white'
                            : 'border-transparent hover:scale-110'
                    "
                    :style="{ backgroundColor: preset }"
                    :title="preset"
                    @click="pick(preset)"
                />
            </div>
            <div class="mt-3 flex items-center gap-2">
                <span
                    class="h-5 w-5 shrink-0 rounded-full ring-1 ring-white/10"
                    :style="{ backgroundColor: parsedHex ?? color }"
                />
                <input
                    v-model="hexInput"
                    type="text"
                    maxlength="7"
                    placeholder="#RRGGBB"
                    spellcheck="false"
                    class="border-border bg-void/80 focus:border-lava/40 min-w-0 flex-1 rounded-md
                        border px-2 py-1 font-mono text-[11px] text-white uppercase
                        focus:outline-none"
                    :class="hexInput && !parsedHex ? 'border-lava/60' : ''"
                    @keydown.enter.prevent="applyHex"
                />
                <button
                    type="button"
                    :disabled="!parsedHex"
                    class="bg-lava hover:bg-lava-glow rounded-md px-2 py-1 text-[11px] font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                    @click="applyHex"
                >
                    Set
                </button>
            </div>
        </div>
    </div>
</template>
