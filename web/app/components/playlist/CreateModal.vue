<script setup lang="ts">
import type { PlaylistListItem, CreatePlaylistInput } from '~/types/playlist';

interface TagItem {
    id: number;
    name: string;
    color: string;
}

const props = defineProps<{
    visible: boolean;
    prefillName?: string;
    prefillSceneIds?: number[];
}>();

const emit = defineEmits<{
    close: [];
    created: [playlist: PlaylistListItem];
}>();

const { createPlaylist, fetchPlaylists, addScenes } = useApiPlaylists();
const { fetchTags } = useApiTags();

const hasScenes = computed(() => (props.prefillSceneIds?.length ?? 0) > 0);

// Tab: 'existing' (add to existing) or 'new' (create new)
const activeTab = ref<'existing' | 'new'>('existing');

// Create form state
const name = ref('');
const description = ref('');
const visibility = ref('public');
const selectedTagIds = ref<number[]>([]);
const loading = ref(false);
const error = ref('');
const success = ref('');

// Tags state
const availableTags = ref<TagItem[]>([]);

// Existing playlists state
const existingPlaylists = ref<PlaylistListItem[]>([]);
const loadingPlaylists = ref(false);
const actionLoading = ref(false);

const loadTags = async () => {
    try {
        const result = await fetchTags();
        availableTags.value = result.data;
    } catch {
        availableTags.value = [];
    }
};

const loadExistingPlaylists = async () => {
    loadingPlaylists.value = true;
    try {
        const result = await fetchPlaylists({ owner: 'me', limit: 50 });
        existingPlaylists.value = result.data;
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load playlists';
    } finally {
        loadingPlaylists.value = false;
    }
};

watch(
    () => props.visible,
    async (visible) => {
        if (visible) {
            name.value = props.prefillName || '';
            description.value = '';
            visibility.value = 'public';
            selectedTagIds.value = [];
            error.value = '';
            success.value = '';
            activeTab.value = hasScenes.value ? 'existing' : 'new';

            if (hasScenes.value) {
                await loadExistingPlaylists();
            }
            await loadTags();
        }
    },
    { immediate: true },
);

const handleAddToExisting = async (playlistUuid: string, playlistName: string) => {
    if (!props.prefillSceneIds?.length) return;

    actionLoading.value = true;
    error.value = '';
    success.value = '';

    try {
        await addScenes(playlistUuid, props.prefillSceneIds);
        success.value = `Added ${props.prefillSceneIds.length} scene(s) to "${playlistName}"`;
        setTimeout(() => {
            emit('created', existingPlaylists.value.find((p) => p.uuid === playlistUuid)!);
        }, 800);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to add scenes';
    } finally {
        actionLoading.value = false;
    }
};

const handleSubmit = async () => {
    if (!name.value.trim()) {
        error.value = 'Please enter a name';
        return;
    }

    error.value = '';
    loading.value = true;

    try {
        const input: CreatePlaylistInput = {
            name: name.value.trim(),
            visibility: visibility.value,
        };
        if (description.value.trim()) {
            input.description = description.value.trim();
        }
        if (selectedTagIds.value.length > 0) {
            input.tag_ids = selectedTagIds.value;
        }
        if (props.prefillSceneIds?.length) {
            input.scene_ids = props.prefillSceneIds;
        }

        const result = await createPlaylist(input);
        emit('created', result);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to create playlist';
    } finally {
        loading.value = false;
    }
};

const toggleTag = (tagId: number) => {
    const idx = selectedTagIds.value.indexOf(tagId);
    if (idx === -1) {
        selectedTagIds.value.push(tagId);
    } else {
        selectedTagIds.value.splice(idx, 1);
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
                <h3 class="mb-1 text-sm font-semibold text-white">
                    {{ hasScenes ? 'Add to Playlist' : 'Create Playlist' }}
                </h3>
                <p v-if="hasScenes" class="text-dim mb-4 text-xs">
                    {{ prefillSceneIds!.length }} scene(s) selected
                </p>
                <div v-else class="mb-4" />

                <!-- Tabs (only shown when scenes are provided) -->
                <div v-if="hasScenes" class="border-border mb-4 flex gap-1 border-b pb-2">
                    <button
                        class="rounded-md px-3 py-1 text-xs font-medium transition-all"
                        :class="
                            activeTab === 'existing'
                                ? 'bg-lava/10 text-lava'
                                : 'text-dim hover:text-white'
                        "
                        @click="activeTab = 'existing'"
                    >
                        Existing Playlist
                    </button>
                    <button
                        class="rounded-md px-3 py-1 text-xs font-medium transition-all"
                        :class="
                            activeTab === 'new'
                                ? 'bg-lava/10 text-lava'
                                : 'text-dim hover:text-white'
                        "
                        @click="activeTab = 'new'"
                    >
                        New Playlist
                    </button>
                </div>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <div
                    v-if="success"
                    class="mb-3 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-2
                        text-xs text-emerald-400"
                >
                    {{ success }}
                </div>

                <!-- Existing playlists tab -->
                <div v-if="hasScenes && activeTab === 'existing'">
                    <div v-if="loadingPlaylists" class="flex justify-center py-8">
                        <LoadingSpinner size="sm" />
                    </div>

                    <template v-else>
                        <div
                            v-if="existingPlaylists.length === 0"
                            class="text-dim py-6 text-center text-xs"
                        >
                            No playlists yet.
                            <button
                                class="text-lava hover:text-lava-glow ml-1"
                                @click="activeTab = 'new'"
                            >
                                Create one
                            </button>
                        </div>
                        <div v-else class="max-h-72 space-y-1 overflow-y-auto">
                            <button
                                v-for="p in existingPlaylists"
                                :key="p.uuid"
                                :disabled="actionLoading"
                                class="border-border bg-surface hover:bg-elevated flex w-full
                                    items-center gap-3 rounded-lg border p-2.5 text-left
                                    transition-all disabled:opacity-50"
                                @click="handleAddToExisting(p.uuid, p.name)"
                            >
                                <div class="bg-void h-8 w-8 shrink-0 overflow-hidden rounded">
                                    <img
                                        v-if="p.thumbnail_scenes.length > 0"
                                        :src="`/thumbnails/${p.thumbnail_scenes[0]?.id}`"
                                        class="h-full w-full object-cover"
                                        alt=""
                                        loading="lazy"
                                    />
                                    <div
                                        v-else
                                        class="flex h-full w-full items-center justify-center"
                                    >
                                        <Icon
                                            name="heroicons:queue-list"
                                            size="12"
                                            class="text-dim"
                                        />
                                    </div>
                                </div>
                                <div class="min-w-0 flex-1">
                                    <div class="truncate text-xs font-medium text-white">
                                        {{ p.name }}
                                    </div>
                                    <div class="text-dim text-[10px]">
                                        {{ p.scene_count }} scenes
                                    </div>
                                </div>
                                <Icon name="heroicons:plus" size="16" class="text-dim shrink-0" />
                            </button>
                        </div>
                    </template>

                    <div class="mt-4 flex justify-end">
                        <button
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="emit('close')"
                        >
                            Close
                        </button>
                    </div>
                </div>

                <!-- Create new playlist form -->
                <form
                    v-if="!hasScenes || activeTab === 'new'"
                    class="space-y-4"
                    @submit.prevent="handleSubmit"
                >
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
                            placeholder="My Playlist"
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

                    <div
                        v-if="prefillSceneIds?.length"
                        class="text-dim bg-void/50 border-border rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ prefillSceneIds.length }} scene(s) will be added
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
                            {{ loading ? 'Creating...' : 'Create Playlist' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
