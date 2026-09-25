<script setup lang="ts">
// A thin progress bar across the bottom of a short. Click or drag to seek.
const props = defineProps<{
    currentTime: number;
    duration: number;
}>();

const emit = defineEmits<{
    seek: [time: number];
}>();

const bar = ref<HTMLElement | null>(null);
const dragging = ref(false);
const dragFraction = ref(0);

const fraction = computed(() => {
    if (dragging.value) return dragFraction.value;
    if (!props.duration) return 0;
    return Math.min(1, Math.max(0, props.currentTime / props.duration));
});

const fractionAt = (clientX: number) => {
    const rect = bar.value?.getBoundingClientRect();
    if (!rect || rect.width === 0) return 0;
    return Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
};

const onPointerDown = (e: PointerEvent) => {
    if (!props.duration) return;
    dragging.value = true;
    dragFraction.value = fractionAt(e.clientX);
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    emit('seek', dragFraction.value * props.duration);
};

const onPointerMove = (e: PointerEvent) => {
    if (!dragging.value) return;
    dragFraction.value = fractionAt(e.clientX);
    emit('seek', dragFraction.value * props.duration);
};

const onPointerUp = (e: PointerEvent) => {
    if (!dragging.value) return;
    dragging.value = false;
    (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
};
</script>

<template>
    <div
        ref="bar"
        class="group flex h-5 cursor-pointer touch-none items-end"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
    >
        <div
            class="relative h-1 w-full bg-white/15 transition-all group-hover:h-1.5"
            :class="{ 'h-1.5': dragging }"
        >
            <div
                class="bg-lava absolute inset-y-0 left-0 shadow-[0_0_10px_rgba(255,77,77,0.5)]"
                :style="{ width: `${fraction * 100}%` }"
            />
        </div>
    </div>
</template>
