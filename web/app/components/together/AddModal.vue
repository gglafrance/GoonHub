<script setup lang="ts">
import type { LobbySummary, QueueItemInput } from '~/types/together';

// "Watch together" from the rest of the app (watch page, selection, markers): start a lobby
// with the items, or add them to one of the user's lobbies.
const props = defineProps<{
    visible: boolean;
    items: QueueItemInput[];
}>();

const emit = defineEmits<{
    close: [];
}>();

const perms = useMyPermissions();
const { listLobbies, addToQueue } = useApiTogether();

const lobbies = ref<LobbySummary[]>([]);
const loading = ref(false);
const error = ref('');
const done = ref<Record<string, string>>({});
const busy = ref<string | null>(null);
const showCreate = ref(false);

const canHost = computed(() => perms.has('goon2gether:host'));
const count = computed(() => props.items.length);

watch(
    () => props.visible,
    async (v) => {
        if (!v) return;
        showCreate.value = false;
        error.value = '';
        done.value = {};
        loading.value = true;
        try {
            const res = await listLobbies();
            lobbies.value = (res.lobbies ?? []).filter((l) => l.is_member);
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load your lobbies';
        } finally {
            loading.value = false;
        }
    },
    { immediate: true },
);

const add = async (lobby: LobbySummary) => {
    if (busy.value) return;
    busy.value = lobby.id;
    error.value = '';
    try {
        const res = await addToQueue(lobby.id, props.items);
        done.value = {
            ...done.value,
            [lobby.id]: res.suggested > 0 && res.queued === 0 ? 'Suggested' : 'Added',
        };
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to add to the lobby';
    } finally {
        busy.value = null;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && !showCreate"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="emit('close')"
            @keydown.esc="emit('close')"
        >
            <div
                class="border-border bg-panel mx-4 w-full max-w-sm rounded-xl border p-5 shadow-2xl"
            >
                <div class="mb-4 flex items-center gap-2">
                    <Icon name="heroicons:user-group" size="16" class="text-lava" />
                    <h3 class="text-sm font-semibold text-white">Watch together</h3>
                    <span class="text-dim ml-auto text-[11px]">
                        {{ count }} item{{ count > 1 ? 's' : '' }}
                    </span>
                </div>

                <button
                    v-if="canHost"
                    class="border-lava/30 bg-lava/10 hover:bg-lava/20 mb-4 flex w-full items-center
                        gap-2 rounded-lg border px-3 py-2 text-left text-xs font-medium text-white
                        transition-colors"
                    @click="showCreate = true"
                >
                    <Icon name="heroicons:plus-circle" size="15" class="text-lava" />
                    Start a lobby with {{ count > 1 ? 'these' : 'this' }}
                </button>

                <div class="text-dim mb-2 text-[10px] font-semibold tracking-wider uppercase">
                    Add to a lobby
                </div>
                <div v-if="loading" class="flex justify-center py-4">
                    <LoadingSpinner />
                </div>
                <div v-else-if="lobbies.length > 0" class="max-h-60 space-y-1 overflow-y-auto">
                    <div
                        v-for="l in lobbies"
                        :key="l.id"
                        class="border-border flex items-center gap-2 rounded-lg border px-3 py-2"
                    >
                        <div class="min-w-0 flex-1">
                            <div class="truncate text-xs text-white">{{ l.name }}</div>
                            <div class="text-dim text-[10px]">
                                {{ l.online_count }} online - {{ l.queue_length }} queued
                            </div>
                        </div>
                        <span v-if="done[l.id]" class="text-emerald text-[11px]">
                            {{ done[l.id] }}
                        </span>
                        <button
                            v-else
                            class="border-border hover:border-lava/30 hover:text-lava rounded-md
                                border px-2 py-1 text-[11px] text-white transition-all
                                disabled:opacity-50"
                            :disabled="busy !== null"
                            @click="add(l)"
                        >
                            Add
                        </button>
                    </div>
                </div>
                <p v-else class="text-dim py-3 text-center text-[11px]">
                    You are not in any lobby.
                </p>

                <p v-if="error" class="text-lava mt-3 text-[11px]">{{ error }}</p>
                <div class="mt-4 flex justify-end">
                    <button
                        class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                            transition-colors hover:text-white"
                        @click="emit('close')"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
    <TogetherLobbyCreateModal
        :visible="visible && showCreate"
        :items="items"
        @close="showCreate = false"
        @created="emit('close')"
    />
</template>
