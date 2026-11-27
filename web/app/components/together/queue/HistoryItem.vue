<script setup lang="ts">
import type { HistoryEntry } from '~/types/together';

// One history row, like a queue row: thumbnail, title, range, who added it, when it ended and
// how (played to the end or skipped), and the requeue actions allowed. A scene that cannot be
// queued anymore (trashed) shows as unavailable, without actions.
const props = defineProps<{
    entry: HistoryEntry;
    // may add scenes (scenes:view); canQueue: to the queue, else as a suggestion
    canAdd: boolean;
    canQueue: boolean;
    canControl: boolean;
}>();

const emit = defineEmits<{
    add: [entry: HistoryEntry];
    playNow: [entry: HistoryEntry];
}>();

const range = computed(() => itemRangeText(props.entry));
const completed = computed(() => props.entry.outcome === 'completed');
</script>

<template>
    <div
        class="group flex items-center gap-2 rounded-lg border border-transparent px-2 py-1.5
            transition-colors hover:bg-white/[0.03]"
        :class="entry.available ? '' : 'opacity-60'"
    >
        <div class="bg-void relative h-9 w-16 shrink-0 overflow-hidden rounded">
            <img
                v-if="entry.available"
                :src="`/thumbnails/${entry.scene_id}?size=sm`"
                alt=""
                class="h-full w-full object-cover"
                loading="lazy"
            />
            <div v-else class="flex h-full w-full items-center justify-center">
                <Icon name="heroicons:eye-slash" size="14" class="text-dim" />
            </div>
        </div>
        <div class="min-w-0 flex-1">
            <div
                class="truncate text-xs"
                :class="entry.available ? 'text-white' : 'text-dim italic'"
                :title="entry.available ? entry.title : 'This video is no longer available'"
            >
                {{ entry.available ? entry.title || 'Untitled video' : 'Unavailable video' }}
            </div>
            <div class="text-dim flex items-center gap-1.5 truncate text-[10px]">
                <span v-if="entry.label" class="text-lava/80 truncate">{{ entry.label }}</span>
                <span v-if="range" class="font-mono">{{ range }}</span>
                <span v-if="entry.added_by" class="truncate">by {{ entry.added_by.username }}</span>
            </div>
            <div class="text-dim flex items-center gap-1 text-[10px]">
                <Icon
                    :name="completed ? 'heroicons:check-circle' : 'heroicons:forward'"
                    size="11"
                    :class="completed ? 'text-emerald' : 'text-dim'"
                />
                <span>{{ completed ? 'Played' : 'Skipped' }}</span>
                <NuxtTime :datetime="entry.ended_at" relative class="truncate" />
            </div>
        </div>
        <div
            v-if="entry.available && (canAdd || canControl)"
            class="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity
                group-focus-within:opacity-100 group-hover:opacity-100"
        >
            <button
                v-if="canControl && canAdd"
                class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center rounded"
                title="Play now"
                @click="emit('playNow', entry)"
            >
                <Icon name="heroicons:play" size="13" />
            </button>
            <button
                v-if="canAdd"
                class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center rounded"
                :title="canQueue ? 'Add to the queue' : 'Suggest'"
                @click="emit('add', entry)"
            >
                <Icon :name="canQueue ? 'heroicons:plus' : 'heroicons:light-bulb'" size="13" />
            </button>
        </div>
    </div>
</template>
