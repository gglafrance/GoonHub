<script setup lang="ts">
import type { PickedItem } from '~/utils/picker';

// Playing queue: now playing, upcoming items (drag to reorder for controllers), suggestions
// and the search picker to add videos and markers (or suggest them, without queue rights).
const room = useInjectedTogetherRoom();
const { socket, canControl, canQueue, canAddScenes, currentItem, upcoming } = room;

const showPicker = ref(false);
const dragFrom = ref<number | null>(null);
const dropAt = ref<number | null>(null);

// Videos already queued whole are marked in the picker (they can still be queued again)
const queuedScenes = computed(() => ({
    scenes: socket.queue.value
        .filter((i) => i.start == null && i.end == null)
        .map((i) => i.scene_id),
}));
// "Play now": controllers only (the server refuses it otherwise)
const pickerActions = computed(() =>
    canControl.value ? [{ id: 'play', label: 'Play now', icon: 'heroicons:play' }] : [],
);
// Picked videos are queued whole, markers as their range with their label
const onPicked = (items: PickedItem[], action: string) => {
    const queueItems = pickedToQueueItems(items);
    if (queueItems.length === 0) return;
    if (action === 'play') socket.send({ t: 'queue_add', items: queueItems, play_now: true });
    else socket.send({ t: 'queue_add', items: queueItems });
    showPicker.value = false;
};
const remove = (id: number) => socket.send({ t: 'queue_remove', item_id: id });
const playNow = (id: number) => socket.send({ t: 'play_item', item_id: id });
const skip = () => socket.send({ t: 'skip' });

const onDragStart = (index: number, e: DragEvent) => {
    if (!canControl.value) return;
    dragFrom.value = index;
    e.dataTransfer?.setData('text/plain', String(index));
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
};
const onDragOver = (index: number, e: DragEvent) => {
    if (dragFrom.value === null) return;
    e.preventDefault();
    const el = e.currentTarget as HTMLElement;
    const rect = el.getBoundingClientRect();
    dropAt.value = e.clientY > rect.top + rect.height / 2 ? index + 1 : index;
};
const onDrop = () => {
    const from = dragFrom.value;
    const to = dropAt.value;
    dragFrom.value = dropAt.value = null;
    if (from === null || to === null) return;
    const item = upcoming.value[from];
    // Index among the upcoming items once the dragged one is taken out
    const target = to > from ? to - 1 : to;
    if (!item || target === from) return;
    socket.send({ t: 'queue_move', item_id: item.id, index: target });
};
const onDragEnd = () => {
    dragFrom.value = dropAt.value = null;
};
</script>

<template>
    <div class="border-border bg-panel/40 rounded-xl border">
        <div class="border-border flex items-center gap-2 border-b px-3 py-2">
            <Icon name="heroicons:queue-list" size="13" class="text-lava" />
            <span class="text-xs font-semibold text-white">Queue</span>
            <span class="text-dim font-mono text-[10px]">{{ socket.queue.value.length }}</span>
            <div class="ml-auto flex items-center gap-1">
                <button
                    v-if="canControl && currentItem"
                    class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-6
                        items-center gap-1 rounded-md border px-2 text-[10px] font-medium
                        transition-all"
                    title="Skip to the next item"
                    @click="skip"
                >
                    <Icon name="heroicons:forward" size="11" />
                    Skip
                </button>
                <button
                    v-if="canAddScenes"
                    class="bg-lava hover:bg-lava/90 flex h-7 items-center gap-1.5 rounded-md px-2.5
                        text-[11px] font-semibold text-white transition-colors"
                    @click="showPicker = true"
                >
                    <Icon name="heroicons:plus" size="13" />
                    {{ canQueue ? 'Add to queue' : 'Suggest' }}
                </button>
            </div>
        </div>

        <div class="space-y-2 p-2">
            <div v-if="currentItem">
                <div class="text-dim mb-1 px-1 text-[10px] font-semibold tracking-wider uppercase">
                    Now playing
                </div>
                <TogetherQueueItem :item="currentItem" current :can-control="canControl" />
            </div>

            <div v-if="upcoming.length > 0">
                <div class="text-dim mb-1 px-1 text-[10px] font-semibold tracking-wider uppercase">
                    Up next
                </div>
                <div class="max-h-72 space-y-0.5 overflow-y-auto" @dragleave.self="dropAt = null">
                    <div
                        v-for="(item, i) in upcoming"
                        :key="item.id"
                        class="relative"
                        @dragstart="onDragStart(i, $event)"
                        @dragover="onDragOver(i, $event)"
                        @drop.prevent="onDrop"
                        @dragend="onDragEnd"
                    >
                        <div
                            v-if="dropAt === i"
                            class="bg-lava absolute -top-px right-0 left-0 h-0.5 rounded"
                        />
                        <TogetherQueueItem
                            :item="item"
                            :index="i"
                            :draggable="canControl"
                            :can-control="canControl"
                            @play-now="playNow"
                            @remove="remove"
                        />
                        <div
                            v-if="dropAt === i + 1 && i === upcoming.length - 1"
                            class="bg-lava absolute right-0 -bottom-px left-0 h-0.5 rounded"
                        />
                    </div>
                </div>
            </div>

            <div
                v-if="!currentItem && upcoming.length === 0"
                class="flex flex-col items-center gap-2 py-5 text-center"
            >
                <p class="text-dim text-[11px]">Nothing queued yet.</p>
                <button
                    v-if="canAddScenes"
                    class="border-lava/30 bg-lava/10 hover:bg-lava/20 flex items-center gap-1.5
                        rounded-lg border px-3 py-1.5 text-xs font-medium text-white
                        transition-colors"
                    @click="showPicker = true"
                >
                    <Icon name="heroicons:magnifying-glass" size="13" class="text-lava" />
                    {{
                        canQueue ? 'Find videos and markers to queue' : 'Suggest videos and markers'
                    }}
                </button>
            </div>

            <TogetherQueueSuggestions />
        </div>

        <PickerModal
            :visible="showPicker"
            multiple
            :max="100"
            :exclude="queuedScenes"
            excluded-selectable
            excluded-label="In queue"
            :title="canQueue ? 'Add to the queue' : 'Suggest to the queue'"
            :confirm-label="canQueue ? 'Add' : 'Suggest'"
            :actions="pickerActions"
            :hint="canQueue ? undefined : 'The host or a controller accepts suggestions'"
            @picked="onPicked"
            @close="showPicker = false"
        />
    </div>
</template>
