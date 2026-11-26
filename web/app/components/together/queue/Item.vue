<script setup lang="ts">
import type { QueueItem } from '~/types/together';

// One queue row: thumbnail, title, range or duration, who added it, and the actions allowed.
const props = defineProps<{
    item: QueueItem;
    current?: boolean;
    index?: number;
    draggable?: boolean;
    canControl: boolean;
    canRemoveOwn?: boolean;
}>();

const emit = defineEmits<{
    playNow: [id: number];
    remove: [id: number];
}>();

const range = computed(() => {
    const { start, end, duration } = props.item;
    if (start != null || end != null) {
        return `${formatClock(start ?? 0)} - ${end != null ? formatClock(end) : formatClock(duration)}`;
    }
    return duration > 0 ? formatClock(duration) : '';
});
</script>

<template>
    <div
        class="group flex items-center gap-2 rounded-lg border px-2 py-1.5 transition-colors"
        :class="[
            current ? 'border-lava/30 bg-lava/5' : 'border-transparent hover:bg-white/[0.03]',
            draggable ? 'cursor-grab active:cursor-grabbing' : '',
        ]"
        :draggable="draggable"
    >
        <span
            v-if="index !== undefined"
            class="text-dim w-4 shrink-0 text-right font-mono text-[10px]"
        >
            {{ index + 1 }}
        </span>
        <div class="bg-void relative h-9 w-16 shrink-0 overflow-hidden rounded">
            <img
                :src="`/thumbnails/${item.scene_id}?size=sm`"
                alt=""
                class="h-full w-full object-cover"
                loading="lazy"
            />
            <div
                v-if="current"
                class="absolute inset-0 flex items-center justify-center bg-black/40"
            >
                <Icon name="heroicons:play-solid" size="14" class="text-lava" />
            </div>
        </div>
        <div class="min-w-0 flex-1">
            <div class="truncate text-xs text-white" :title="item.title">{{ item.title }}</div>
            <div class="text-dim flex items-center gap-1.5 truncate text-[10px]">
                <span v-if="item.label" class="text-lava/80 truncate">{{ item.label }}</span>
                <span v-if="range" class="font-mono">{{ range }}</span>
                <span v-if="item.added_by" class="truncate">by {{ item.added_by.username }}</span>
            </div>
        </div>
        <div
            class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity
                group-hover:opacity-100"
        >
            <button
                v-if="canControl && !current"
                class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center rounded"
                title="Play now"
                @click="emit('playNow', item.id)"
            >
                <Icon name="heroicons:play" size="13" />
            </button>
            <button
                v-if="(canControl || canRemoveOwn) && !current"
                class="text-dim flex h-6 w-6 items-center justify-center rounded hover:text-red-400"
                title="Remove"
                @click="emit('remove', item.id)"
            >
                <Icon name="heroicons:x-mark" size="13" />
            </button>
        </div>
    </div>
</template>
