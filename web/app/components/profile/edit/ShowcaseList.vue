<script setup lang="ts">
import type { ShowcaseItem } from '~/types/profile';

// The items of a showcase being edited: thumbnails in order, drag and drop (or arrows) to
// reorder, replace (the parent opens the picker), remove. Emits the new order of ids.

const props = defineProps<{ items: ShowcaseItem[]; max: number; disabled?: boolean }>();
const emit = defineEmits<{ change: [ids: number[]]; replace: [index: number] }>();

const ids = () => props.items.map((i) => i.id);
const move = (from: number, to: number) => emit('change', moveItem(ids(), from, to));
const drag = useDragReorder(move);
const remove = (id: number) =>
    emit(
        'change',
        ids().filter((x) => x !== id),
    );
</script>

<template>
    <div>
        <div class="text-dim mb-1.5 text-[10px]">{{ items.length }}/{{ max }}</div>
        <div
            v-if="items.length"
            class="grid grid-cols-2 gap-2 sm:grid-cols-4"
            :class="disabled ? 'pointer-events-none opacity-60' : ''"
        >
            <div
                v-for="(item, i) in items"
                :key="item.id"
                v-bind="drag.handlers(i)"
                class="group border-border relative cursor-grab overflow-hidden rounded-lg border
                    bg-black/40"
                :class="drag.overIndex.value === i ? 'border-lava/60' : ''"
            >
                <div class="aspect-video w-full">
                    <img
                        v-if="item.thumb"
                        :src="item.thumb"
                        alt=""
                        class="h-full w-full object-cover"
                        draggable="false"
                    />
                </div>
                <div class="truncate px-1.5 py-1 text-[10px] text-white">
                    {{ i + 1 }}. {{ item.title }}
                </div>
                <div
                    class="absolute top-1 right-1 flex gap-1 opacity-0 transition-opacity
                        group-hover:opacity-100 focus-within:opacity-100"
                >
                    <button
                        class="rounded bg-black/70 p-0.5 text-white disabled:opacity-30"
                        :disabled="i === 0"
                        title="Move left"
                        @click="move(i, i - 1)"
                    >
                        <Icon name="heroicons:chevron-left" size="12" />
                    </button>
                    <button
                        class="rounded bg-black/70 p-0.5 text-white disabled:opacity-30"
                        :disabled="i === items.length - 1"
                        title="Move right"
                        @click="move(i, i + 1)"
                    >
                        <Icon name="heroicons:chevron-right" size="12" />
                    </button>
                    <button
                        class="hover:text-lava rounded bg-black/70 p-0.5 text-white"
                        title="Replace"
                        @click="emit('replace', i)"
                    >
                        <Icon name="heroicons:arrow-path" size="12" />
                    </button>
                    <button
                        class="hover:text-lava rounded bg-black/70 p-0.5 text-white"
                        title="Remove"
                        @click="remove(item.id)"
                    >
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </div>
            </div>
        </div>
        <p
            v-else
            class="text-dim border-border rounded-lg border border-dashed px-3 py-4 text-center
                text-[11px]"
        >
            Nothing picked yet.
        </p>
    </div>
</template>
