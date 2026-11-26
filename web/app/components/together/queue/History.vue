<script setup lang="ts">
import type { HistoryEntry } from '~/types/together';

// The previously played items of the lobby, newest first, with the requeue actions: add to
// the queue (a suggestion without queue rights), play now (control), and requeue all. They
// reuse queue_add with the same scene, range and label.
const room = useInjectedTogetherRoom();
const { socket, canControl, canQueue, canAddScenes } = room;

const entries = computed(() => socket.history.value);
const requeueable = computed(() => requeueAllItems(entries.value));

// "Requeue all" asks once more before adding many items
const confirmingAll = ref(false);
let confirmTimer: ReturnType<typeof setTimeout> | null = null;
const resetConfirm = () => {
    confirmingAll.value = false;
    if (confirmTimer) clearTimeout(confirmTimer);
    confirmTimer = null;
};
onBeforeUnmount(resetConfirm);

const add = (entry: HistoryEntry) =>
    socket.send({ t: 'queue_add', items: [historyEntryToQueueItem(entry)] });
const playNow = (entry: HistoryEntry) =>
    socket.send({ t: 'queue_add', items: [historyEntryToQueueItem(entry)], play_now: true });
const requeueAll = () => {
    if (requeueable.value.length === 0) return;
    if (!confirmingAll.value && requeueable.value.length > 1) {
        confirmingAll.value = true;
        confirmTimer = setTimeout(resetConfirm, 4000);
        return;
    }
    resetConfirm();
    socket.send({ t: 'queue_add', items: requeueable.value });
};
</script>

<template>
    <div class="space-y-1 p-2">
        <div
            v-if="entries.length === 0"
            class="text-dim flex flex-col items-center gap-1 py-5 text-center text-[11px]"
        >
            <Icon name="heroicons:clock" size="16" class="text-dim/60" />
            Nothing played yet. Finished and skipped videos show up here.
        </div>
        <template v-else>
            <div class="flex items-center gap-2 px-1">
                <span class="text-dim text-[10px] font-semibold tracking-wider uppercase">
                    Previously played
                </span>
                <button
                    v-if="canAddScenes && requeueable.length > 0"
                    class="ml-auto flex h-6 items-center gap-1 rounded-md border px-2 text-[10px]
                        font-medium transition-all"
                    :class="
                        confirmingAll
                            ? 'border-lava/40 bg-lava/10 text-white'
                            : 'border-border text-dim hover:border-lava/30 hover:text-lava'
                    "
                    :title="
                        canQueue
                            ? 'Add every available video to the end of the queue, once each'
                            : 'Suggest every available video, once each'
                    "
                    @click="requeueAll"
                >
                    <Icon name="heroicons:arrow-path" size="11" />
                    <template v-if="confirmingAll">
                        {{ canQueue ? 'Add' : 'Suggest' }} {{ requeueable.length }} videos?
                    </template>
                    <template v-else>{{ canQueue ? 'Requeue all' : 'Suggest all' }}</template>
                </button>
            </div>
            <div class="max-h-72 space-y-0.5 overflow-y-auto">
                <TogetherQueueHistoryItem
                    v-for="entry in entries"
                    :key="entry.id"
                    :entry="entry"
                    :can-add="canAddScenes"
                    :can-queue="canQueue"
                    :can-control="canControl"
                    @add="add"
                    @play-now="playNow"
                />
            </div>
        </template>
    </div>
</template>
