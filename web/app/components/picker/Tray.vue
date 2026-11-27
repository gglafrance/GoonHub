<script setup lang="ts">
import type { PickedItem } from '~/utils/picker';

// Selection of the search picker: the picked items in order (thumbnail and title), each can be
// removed; "Clear" empties the selection.
const props = defineProps<{ items: PickedItem[]; max?: number }>();
const emit = defineEmits<{ remove: [item: PickedItem]; clear: [] }>();

const entries = computed(() =>
    props.items.map((item) => ({ item, key: pickerKey(item), ...pickedSummary(item) })),
);
</script>

<template>
    <div class="flex min-w-0 flex-1 items-center gap-2">
        <span class="text-dim shrink-0 font-mono text-[11px]">
            {{ items.length }}{{ max ? `/${max}` : '' }} selected
        </span>
        <div class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto py-0.5">
            <div
                v-for="e in entries"
                :key="e.key"
                class="border-border bg-surface group flex h-8 max-w-48 shrink-0 items-center
                    gap-1.5 rounded-md border pr-1"
                :title="e.title"
            >
                <img
                    v-if="e.thumb"
                    :src="e.thumb"
                    alt=""
                    class="h-full w-12 shrink-0 rounded-l-md object-cover"
                    loading="lazy"
                />
                <Icon
                    v-else
                    :name="e.item.kind === 'scene' ? 'heroicons:film' : 'heroicons:bookmark'"
                    size="13"
                    class="text-dim ml-1.5 shrink-0"
                />
                <span class="truncate text-[11px] text-white">{{ e.title }}</span>
                <button
                    class="text-dim hover:text-lava shrink-0 rounded p-0.5"
                    title="Remove from the selection"
                    @click="emit('remove', e.item)"
                >
                    <Icon name="heroicons:x-mark" size="12" />
                </button>
            </div>
        </div>
        <button
            v-if="items.length > 1"
            class="text-dim shrink-0 text-[11px] transition-colors hover:text-white"
            @click="emit('clear')"
        >
            Clear
        </button>
    </div>
</template>
