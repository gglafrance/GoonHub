<script setup lang="ts">
import type { ImageCrop } from '~/types/profile';

// Crop of an avatar (aspect 1) or banner (aspect 3) before the upload: drag to move, slider
// to zoom. Emits the crop rectangle in source pixels; the server crops and resizes.
const props = defineProps<{ visible: boolean; file: File | null; aspect: number; title: string }>();
const emit = defineEmits<{ close: []; confirm: [crop: ImageCrop] }>();

const FRAME_W = 320;
const frame = computed(() => ({ width: FRAME_W, height: Math.round(FRAME_W / props.aspect) }));

const url = ref('');
const natural = ref({ width: 0, height: 0 });
const zoom = ref(1);
const offset = ref({ x: 0, y: 0 });
let drag: { x: number; y: number; ox: number; oy: number } | null = null;

const baseScale = computed(() =>
    natural.value.width
        ? Math.max(
              frame.value.width / natural.value.width,
              frame.value.height / natural.value.height,
          )
        : 1,
);
const scale = computed(() => baseScale.value * zoom.value);
const scaled = computed(() => ({
    width: natural.value.width * scale.value,
    height: natural.value.height * scale.value,
}));

function clampAll() {
    offset.value = {
        x: clampOffset(offset.value.x, scaled.value.width, frame.value.width),
        y: clampOffset(offset.value.y, scaled.value.height, frame.value.height),
    };
}

function center() {
    offset.value = {
        x: (frame.value.width - scaled.value.width) / 2,
        y: (frame.value.height - scaled.value.height) / 2,
    };
}

watch(
    () => props.file,
    (file) => {
        if (url.value) URL.revokeObjectURL(url.value);
        url.value = file ? URL.createObjectURL(file) : '';
        natural.value = { width: 0, height: 0 };
        zoom.value = 1;
    },
    { immediate: true },
);
onBeforeUnmount(() => {
    if (url.value) URL.revokeObjectURL(url.value);
});

function onLoad(e: Event) {
    const img = e.target as HTMLImageElement;
    natural.value = { width: img.naturalWidth, height: img.naturalHeight };
    center();
}

watch(zoom, (z, old) => {
    // Zoom around the frame center
    const cx = frame.value.width / 2;
    const cy = frame.value.height / 2;
    const ratio = z / old;
    offset.value = { x: cx - (cx - offset.value.x) * ratio, y: cy - (cy - offset.value.y) * ratio };
    clampAll();
});

function onPointerDown(e: PointerEvent) {
    drag = { x: e.clientX, y: e.clientY, ox: offset.value.x, oy: offset.value.y };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
}
function onPointerMove(e: PointerEvent) {
    if (!drag) return;
    offset.value = { x: drag.ox + e.clientX - drag.x, y: drag.oy + e.clientY - drag.y };
    clampAll();
}
function onPointerUp() {
    drag = null;
}

function confirm() {
    if (!natural.value.width) return;
    emit(
        'confirm',
        cropFromView(natural.value, frame.value, scale.value, offset.value.x, offset.value.y),
    );
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4
                backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel w-full max-w-sm space-y-3 p-4">
                <h3 class="text-sm font-semibold text-white">{{ title }}</h3>
                <div
                    class="border-border relative mx-auto cursor-grab touch-none overflow-hidden
                        border bg-black select-none active:cursor-grabbing"
                    :class="aspect === 1 ? 'rounded-full' : 'rounded-lg'"
                    :style="{ width: `${frame.width}px`, height: `${frame.height}px` }"
                    @pointerdown="onPointerDown"
                    @pointermove="onPointerMove"
                    @pointerup="onPointerUp"
                    @pointercancel="onPointerUp"
                >
                    <img
                        v-if="url"
                        :src="url"
                        alt=""
                        draggable="false"
                        class="absolute max-w-none"
                        :style="{
                            width: `${scaled.width}px`,
                            height: `${scaled.height}px`,
                            left: `${offset.x}px`,
                            top: `${offset.y}px`,
                        }"
                        @load="onLoad"
                    />
                </div>
                <label class="text-dim flex items-center gap-2 text-[11px]">
                    <Icon name="heroicons:magnifying-glass-minus" size="13" />
                    <input
                        v-model.number="zoom"
                        type="range"
                        min="1"
                        max="4"
                        step="0.01"
                        class="accent-lava flex-1"
                    />
                    <Icon name="heroicons:magnifying-glass-plus" size="13" />
                </label>
                <div class="flex justify-end gap-2">
                    <button
                        class="text-dim rounded-md px-3 py-1.5 text-xs hover:text-white"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                            text-white disabled:opacity-50"
                        :disabled="!natural.width"
                        @click="confirm"
                    >
                        Upload
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
