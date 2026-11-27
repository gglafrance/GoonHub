<script setup lang="ts">
import type { LobbySummary, LobbyVisibility, QueueItemInput } from '~/types/together';

// Creates a lobby (optionally with items to queue) and opens it.
const props = defineProps<{
    visible: boolean;
    items?: QueueItemInput[];
}>();

const emit = defineEmits<{
    close: [];
    created: [lobby: LobbySummary];
}>();

const { createLobby } = useApiTogether();
const authStore = useAuthStore();

const name = ref('');
const visibility = ref<LobbyVisibility>('private');
const controlOpen = ref(false);
const queueOpen = ref(false);
const waitForAll = ref(false);
const maxMembers = ref(0);
const loading = ref(false);
const error = ref('');

watch(
    () => props.visible,
    (v) => {
        if (!v) return;
        name.value = authStore.user ? `${authStore.user.username}'s lobby` : '';
        visibility.value = 'private';
        controlOpen.value = queueOpen.value = waitForAll.value = false;
        maxMembers.value = 0;
        error.value = '';
    },
    { immediate: true },
);

const submit = async () => {
    if (loading.value) return;
    loading.value = true;
    error.value = '';
    try {
        const lobby = await createLobby({
            name: name.value.trim() || undefined,
            visibility: visibility.value,
            control_open: controlOpen.value,
            queue_open: queueOpen.value,
            wait_for_all: waitForAll.value,
            max_members: Math.max(0, Math.floor(maxMembers.value || 0)),
            items: props.items?.length ? props.items : undefined,
        });
        emit('created', lobby);
        await navigateTo(`/together/${lobby.id}`);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to create the lobby';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="emit('close')"
            @keydown.esc="emit('close')"
        >
            <div
                class="border-border bg-panel mx-4 w-full max-w-md rounded-xl border p-5 shadow-2xl"
            >
                <div class="mb-4 flex items-center gap-2">
                    <Icon name="heroicons:user-group" size="16" class="text-lava" />
                    <h3 class="text-sm font-semibold text-white">New lobby</h3>
                    <span v-if="items?.length" class="text-dim ml-auto text-[11px]">
                        {{ items.length }} item{{ items.length > 1 ? 's' : '' }} queued
                    </span>
                </div>
                <TogetherLobbySettingsFields
                    v-model:name="name"
                    v-model:visibility="visibility"
                    v-model:control-open="controlOpen"
                    v-model:queue-open="queueOpen"
                    v-model:wait-for-all="waitForAll"
                    v-model:max-members="maxMembers"
                />
                <p v-if="error" class="text-lava mt-3 text-[11px]">{{ error }}</p>
                <div class="mt-5 flex justify-end gap-2">
                    <button
                        class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                            transition-colors hover:text-white"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        class="bg-lava hover:bg-lava/90 flex items-center gap-1.5 rounded-lg px-3
                            py-1.5 text-xs font-medium text-white transition-colors
                            disabled:opacity-50"
                        :disabled="loading"
                        @click="submit"
                    >
                        <Icon v-if="loading" name="svg-spinners:90-ring-with-bg" size="12" />
                        Create and open
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
