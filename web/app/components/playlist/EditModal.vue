<script setup lang="ts">
import type { PlaylistDetail, UpdatePlaylistInput } from '~/types/playlist';

interface TagItem {
    id: number;
    name: string;
    color: string;
}

const props = defineProps<{
    visible: boolean;
    playlist: PlaylistDetail | null;
}>();

const emit = defineEmits<{
    close: [];
    updated: [];
}>();

const { updatePlaylist, setPlaylistTags } = useApiPlaylists();
const { fetchTags } = useApiTags();

const name = ref('');
const description = ref('');
const visibility = ref('public');
const selectedTagIds = ref<number[]>([]);
const availableTags = ref<TagItem[]>([]);
const loading = ref(false);
const error = ref('');

const loadTags = async () => {
    try {
        const result = await fetchTags();
        availableTags.value = result.data;
    } catch {
        availableTags.value = [];
    }
};

watch(
    () => props.visible,
    async (visible) => {
        if (visible && props.playlist) {
            name.value = props.playlist.name;
            description.value = props.playlist.description || '';
            visibility.value = props.playlist.visibility;
            selectedTagIds.value = props.playlist.tags?.map((t) => t.id) ?? [];
            error.value = '';
            await loadTags();
        }
    },
    { immediate: true },
);

const toggleTag = (tagId: number) => {
    const idx = selectedTagIds.value.indexOf(tagId);
    if (idx === -1) {
        selectedTagIds.value.push(tagId);
    } else {
        selectedTagIds.value.splice(idx, 1);
    }
};

const handleSubmit = async () => {
    if (!props.playlist || !name.value.trim()) {
        error.value = 'Please enter a name';
        return;
    }

    error.value = '';
    loading.value = true;

    try {
        const input: UpdatePlaylistInput = {};
        if (name.value.trim() !== props.playlist.name) {
            input.name = name.value.trim();
        }
        const desc = description.value.trim();
        if (desc !== (props.playlist.description || '')) {
            input.description = desc;
        }
        if (visibility.value !== props.playlist.visibility) {
            input.visibility = visibility.value;
        }

        const currentTagIds = props.playlist.tags?.map((t) => t.id) ?? [];
        const tagsChanged =
            selectedTagIds.value.length !== currentTagIds.length ||
            selectedTagIds.value.some((id) => !currentTagIds.includes(id));

        await updatePlaylist(props.playlist.uuid, input);

        if (tagsChanged) {
            await setPlaylistTags(props.playlist.uuid, selectedTagIds.value);
        }

        emit('updated');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to update playlist';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel border-border my-8 w-full max-w-md border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">Edit Playlist</h3>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <form class="space-y-4" @submit.prevent="handleSubmit">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Name *
                        </label>
                        <input
                            v-model="name"
                            type="text"
                            required
                            maxlength="255"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Description
                        </label>
                        <textarea
                            v-model="description"
                            maxlength="1000"
                            rows="2"
                            placeholder="Optional description..."
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full resize-none rounded-lg border px-3 py-2
                                text-sm text-white transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Visibility
                        </label>
                        <div class="flex gap-2">
                            <button
                                v-for="opt in ['public', 'private']"
                                :key="opt"
                                type="button"
                                class="rounded-lg border px-3 py-1.5 text-xs font-medium
                                    transition-all"
                                :class="
                                    visibility === opt
                                        ? 'border-lava/40 bg-lava/10 text-lava'
                                        : 'border-border bg-void/50 text-dim hover:text-white'
                                "
                                @click="visibility = opt"
                            >
                                {{ opt.charAt(0).toUpperCase() + opt.slice(1) }}
                            </button>
                        </div>
                    </div>

                    <!-- Tag selection -->
                    <div v-if="availableTags.length > 0">
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Tags
                        </label>
                        <div class="flex max-h-24 flex-wrap gap-1.5 overflow-y-auto">
                            <button
                                v-for="tag in availableTags"
                                :key="tag.id"
                                type="button"
                                class="rounded-md border px-2 py-0.5 text-[11px] font-medium
                                    transition-all"
                                :class="
                                    selectedTagIds.includes(tag.id)
                                        ? 'border-transparent'
                                        : 'border-border'
                                "
                                :style="{
                                    backgroundColor: selectedTagIds.includes(tag.id)
                                        ? tag.color + '30'
                                        : '',
                                    color: selectedTagIds.includes(tag.id)
                                        ? tag.color
                                        : 'rgb(var(--color-dim))',
                                }"
                                @click="toggleTag(tag.id)"
                            >
                                {{ tag.name }}
                            </button>
                        </div>
                    </div>

                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !name.trim()"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{ loading ? 'Saving...' : 'Save Changes' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
