<script setup lang="ts">
import type { Emote } from '~/types/together';

// Settings > Emotes (emotes:manage): server custom emotes of the Goon2gether chat. Upload,
// rename, delete (with confirmation: the image file is removed).
const { renameEmote, deleteEmote } = useApiTogether();
const { emotes, load } = useTogetherEmotes();
const { message, error, setMessage, setError } = useSettingsMessage();

const loading = ref(true);
const query = ref('');
const deleting = ref<Emote | null>(null);

onMounted(async () => {
    await load(true);
    loading.value = false;
});

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase();
    const list = [...emotes.value].sort((a, b) => a.name.localeCompare(b.name));
    return q ? list.filter((e) => e.name.toLowerCase().includes(q)) : list;
});

const onUploaded = async (emote: Emote) => {
    setMessage(`Emote :${emote.name}: added`);
    await load(true);
};

const onRename = async (emote: Emote, name: string) => {
    try {
        await renameEmote(emote.id, name);
        setMessage(`Renamed to :${name}:`);
        await load(true);
    } catch (e: unknown) {
        setError(e, 'Failed to rename the emote');
    }
};

const confirmDelete = async () => {
    const emote = deleting.value;
    deleting.value = null;
    if (!emote) return;
    try {
        await deleteEmote(emote.id);
        setMessage(`Emote :${emote.name}: deleted`);
        await load(true);
    } catch (e: unknown) {
        setError(e, 'Failed to delete the emote');
    }
};
</script>

<template>
    <div class="space-y-3">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div class="glass-panel space-y-3 p-5">
            <div>
                <h3 class="text-sm font-semibold text-white">Emotes</h3>
                <p class="text-dim mt-0.5 text-xs">
                    Custom emotes of the Together chat and reactions: type
                    <span class="font-mono text-white/80">:name:</span> in a message.
                </p>
            </div>
            <SettingsEmotesUploadForm @uploaded="onUploaded" @error="setError($event)" />
        </div>

        <div class="glass-panel space-y-3 p-5">
            <div class="flex items-center gap-3">
                <h3 class="text-sm font-semibold text-white">Server emotes</h3>
                <span class="text-dim font-mono text-[11px]">{{ emotes.length }}</span>
                <input
                    v-model="query"
                    type="text"
                    placeholder="Filter"
                    class="border-border bg-surface focus:border-lava/40 ml-auto w-40 rounded-md
                        border px-2 py-1 text-xs text-white outline-none"
                />
            </div>
            <div v-if="loading" class="flex justify-center py-6">
                <LoadingSpinner />
            </div>
            <p v-else-if="filtered.length === 0" class="text-dim py-6 text-center text-xs">
                No emote yet.
            </p>
            <div v-else class="divide-border divide-y">
                <SettingsEmotesRow
                    v-for="e in filtered"
                    :key="e.id"
                    :emote="e"
                    @rename="onRename(e, $event)"
                    @delete="deleting = e"
                />
            </div>
        </div>

        <UiConfirmModal
            :visible="!!deleting"
            title="Delete this emote?"
            confirm-label="Delete"
            @close="deleting = null"
            @confirm="confirmDelete"
        >
            <p>
                <span class="font-mono text-white">:{{ deleting?.name }}:</span> is removed from the
                server and its image deleted. Old messages show the text instead.
            </p>
        </UiConfirmModal>
    </div>
</template>
