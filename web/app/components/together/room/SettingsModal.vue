<script setup lang="ts">
import type { LobbyInfo, LobbyVisibility } from '~/types/together';

// Lobby settings (host): sent over the socket, the server answers with the new `lobby`.
const props = defineProps<{
    visible: boolean;
    lobby: LobbyInfo | null;
}>();

const emit = defineEmits<{
    close: [];
}>();

const { socket } = useInjectedTogetherRoom();

const name = ref('');
const visibility = ref<LobbyVisibility>('private');
const controlOpen = ref(false);
const queueOpen = ref(false);
const waitForAll = ref(false);
const maxMembers = ref(0);
const error = ref('');

watch(
    () => props.visible,
    (v) => {
        if (!v || !props.lobby) return;
        name.value = props.lobby.name;
        visibility.value = props.lobby.visibility;
        controlOpen.value = props.lobby.control_open;
        queueOpen.value = props.lobby.queue_open;
        waitForAll.value = props.lobby.wait_for_all;
        maxMembers.value = props.lobby.max_members;
        error.value = '';
    },
    { immediate: true },
);

const save = () => {
    const trimmed = name.value.trim();
    if (!trimmed) {
        error.value = 'Please enter a name';
        return;
    }
    const sent = socket.send({
        t: 'settings',
        name: trimmed.slice(0, 100),
        visibility: visibility.value,
        control_open: controlOpen.value,
        queue_open: queueOpen.value,
        wait_for_all: waitForAll.value,
        max_members: Math.max(0, Math.floor(maxMembers.value || 0)),
    });
    if (!sent) {
        error.value = 'Not connected';
        return;
    }
    emit('close');
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
                <h3 class="mb-4 text-sm font-semibold text-white">Lobby settings</h3>
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
                        class="bg-lava hover:bg-lava/90 rounded-lg px-3 py-1.5 text-xs font-medium
                            text-white transition-colors"
                        @click="save"
                    >
                        Save
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
