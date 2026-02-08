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
const initialLoad = ref(true);
const actionLoading = ref(false);

const showSkeleton = computed(() => loadingPlaylists.value && initialLoad.value);

const tabOptions = [
    { key: 'existing' as const, label: 'Existing', icon: 'heroicons:queue-list' },
    { key: 'new' as const, label: 'Create New', icon: 'heroicons:plus' },
] as const;

const visibilityOptions = [
    { key: 'public', label: 'Public', icon: 'heroicons:globe-alt' },
    { key: 'private', label: 'Private', icon: 'heroicons:lock-closed' },
] as const;

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
        initialLoad.value = false;
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
            initialLoad.value = true;
            activeTab.value = hasScenes.value ? 'existing' : 'new';

            if (hasScenes.value) {
                await loadExistingPlaylists();
            }
            await loadTags();
        }
    },
    { immediate: true },
);

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') emit('close');
}

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
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="emit('close')"
                @keydown="onKeydown"
            >
                <Transition
                    enter-active-class="transition duration-200 ease-out"
                    enter-from-class="scale-95 opacity-0"
                    enter-to-class="scale-100 opacity-100"
                    leave-active-class="transition duration-150 ease-in"
                    leave-from-class="scale-100 opacity-100"
                    leave-to-class="scale-95 opacity-0"
                    appear
                >
                    <div
                        class="border-border bg-panel flex w-full max-w-md flex-col rounded-xl
                            border shadow-2xl"
                        :class="hasScenes && activeTab === 'existing' ? 'h-[50dvh]' : ''"
                    >
                        <!-- Header -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-b
                                px-4 py-3"
                        >
                            <div class="flex items-center gap-2.5">
                                <div
                                    class="bg-lava/10 flex h-6 w-6 items-center justify-center
                                        rounded-lg"
                                >
                                    <Icon name="heroicons:queue-list" size="13" class="text-lava" />
                                </div>
                                <div>
                                    <h2 class="text-sm font-semibold text-white">
                                        {{ hasScenes ? 'Add to Playlist' : 'Create Playlist' }}
                                    </h2>
                                    <p v-if="hasScenes" class="text-dim text-[10px] leading-tight">
                                        {{ prefillSceneIds!.length }} scene(s) selected
                                    </p>
                                </div>
                            </div>
                            <button
                                class="text-dim flex items-center justify-center rounded-lg p-1.5
                                    transition-colors hover:bg-white/5 hover:text-white"
                                @click="emit('close')"
                            >
                                <Icon name="heroicons:x-mark" size="16" />
                            </button>
                        </div>

                        <!-- Tab selector (only when scenes provided) -->
                        <div v-if="hasScenes" class="border-border shrink-0 border-b px-4 py-2.5">
                            <div class="bg-surface flex gap-0.5 rounded-lg p-0.5">
                                <button
                                    v-for="tab in tabOptions"
                                    :key="tab.key"
                                    class="flex flex-1 items-center justify-center gap-1.5
                                        rounded-md py-1.5 text-[11px] font-medium transition-all"
                                    :class="
                                        activeTab === tab.key
                                            ? 'bg-lava/15 text-lava shadow-sm'
                                            : 'text-dim hover:text-white'
                                    "
                                    @click="activeTab = tab.key"
                                >
                                    <Icon :name="tab.icon" size="12" />
                                    {{ tab.label }}
                                </button>
                            </div>
                        </div>

                        <!-- Error -->
                        <div v-if="error" class="shrink-0 px-4 pt-3 pb-0">
                            <div
                                class="border-lava/20 bg-lava/5 flex items-center gap-2 rounded-lg
                                    border px-3 py-2"
                            >
                                <Icon
                                    name="heroicons:exclamation-triangle"
                                    size="13"
                                    class="text-lava shrink-0"
                                />
                                <span class="text-[11px] text-red-300">{{ error }}</span>
                            </div>
                        </div>

                        <!-- Success -->
                        <div v-if="success" class="shrink-0 px-4 pt-3 pb-0">
                            <div
                                class="flex items-center gap-2 rounded-lg border
                                    border-emerald-500/20 bg-emerald-500/5 px-3 py-2"
                            >
                                <Icon
                                    name="heroicons:check-circle"
                                    size="13"
                                    class="shrink-0 text-emerald-400"
                                />
                                <span class="text-[11px] text-emerald-300">{{ success }}</span>
                            </div>
                        </div>

                        <!-- Existing playlists tab -->
                        <div
                            v-if="hasScenes && activeTab === 'existing'"
                            class="min-h-0 flex-1 overflow-y-auto px-2 py-2"
                        >
                            <!-- Loading skeleton -->
                            <div v-if="showSkeleton" class="space-y-0.5 px-2 py-1">
                                <div
                                    v-for="i in 5"
                                    :key="i"
                                    class="flex items-center gap-2.5 rounded-lg px-2 py-2"
                                >
                                    <div
                                        class="bg-surface h-8 w-8 shrink-0 animate-pulse rounded"
                                    />
                                    <div class="flex-1 space-y-1.5">
                                        <div
                                            class="bg-surface h-3 animate-pulse rounded"
                                            :style="{ width: `${40 + Math.random() * 40}%` }"
                                        />
                                        <div class="bg-surface h-2 w-14 animate-pulse rounded" />
                                    </div>
                                </div>
                            </div>

                            <!-- Empty state -->
                            <div
                                v-else-if="existingPlaylists.length === 0 && !loadingPlaylists"
                                class="flex flex-col items-center justify-center py-10"
                            >
                                <div
                                    class="bg-surface mb-3 flex h-10 w-10 items-center
                                        justify-center rounded-full"
                                >
                                    <Icon name="heroicons:queue-list" size="18" class="text-dim" />
                                </div>
                                <p class="text-dim text-xs">No playlists yet</p>
                                <button
                                    class="text-lava hover:text-lava-glow mt-1 text-[11px]
                                        font-medium"
                                    @click="activeTab = 'new'"
                                >
                                    Create one
                                </button>
                            </div>

                            <!-- Playlist rows -->
                            <div
                                v-else
                                class="transition-opacity duration-150"
                                :class="loadingPlaylists ? 'pointer-events-none opacity-40' : ''"
                            >
                                <button
                                    v-for="p in existingPlaylists"
                                    :key="p.uuid"
                                    :disabled="actionLoading"
                                    class="group relative flex w-full items-center gap-2.5
                                        rounded-lg px-2 py-2 text-left transition-all
                                        hover:bg-white/3 disabled:opacity-50"
                                    @click="handleAddToExisting(p.uuid, p.name)"
                                >
                                    <!-- Thumbnail -->
                                    <div
                                        class="bg-surface flex h-8 w-8 shrink-0 items-center
                                            justify-center overflow-hidden rounded ring-1
                                            ring-white/6 transition-all group-hover:ring-white/10"
                                    >
                                        <img
                                            v-if="p.thumbnail_scenes.length > 0"
                                            :src="`/thumbnails/${p.thumbnail_scenes[0]?.id}`"
                                            class="h-full w-full object-cover"
                                            alt=""
                                            loading="lazy"
                                        />
                                        <Icon
                                            v-else
                                            name="heroicons:queue-list"
                                            size="12"
                                            class="text-dim"
                                        />
                                    </div>

                                    <!-- Info -->
                                    <div class="min-w-0 flex-1">
                                        <p
                                            class="truncate text-xs font-medium text-white/80
                                                transition-colors group-hover:text-white"
                                        >
                                            {{ p.name }}
                                        </p>
                                        <p class="text-dim text-[10px]">
                                            {{ p.scene_count }}
                                            {{ p.scene_count === 1 ? 'scene' : 'scenes' }}
                                        </p>
                                    </div>

                                    <!-- Add indicator -->
                                    <div
                                        class="bg-lava/5 group-hover:bg-lava/10 flex h-5 w-5
                                            shrink-0 items-center justify-center rounded-full
                                            transition-all"
                                    >
                                        <Icon
                                            name="heroicons:plus"
                                            size="11"
                                            class="text-dim group-hover:text-lava transition-colors"
                                        />
                                    </div>
                                </button>
                            </div>
                        </div>

                        <!-- Create new playlist form -->
                        <div
                            v-if="!hasScenes || activeTab === 'new'"
                            class="flex-1 overflow-y-auto p-4"
                        >
                            <form class="space-y-3.5" @submit.prevent="handleSubmit">
                                <!-- Name -->
                                <div>
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Name *
                                    </label>
                                    <input
                                        v-model="name"
                                        type="text"
                                        required
                                        maxlength="255"
                                        placeholder="My Playlist"
                                        class="border-border bg-surface focus:border-lava/40
                                            focus:ring-lava/10 w-full rounded-lg border px-3 py-2
                                            text-xs text-white placeholder-white/30 transition-all
                                            focus:ring-1 focus:outline-none"
                                    />
                                </div>

                                <!-- Description -->
                                <div>
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Description
                                    </label>
                                    <textarea
                                        v-model="description"
                                        maxlength="1000"
                                        rows="2"
                                        placeholder="Optional description..."
                                        class="border-border bg-surface focus:border-lava/40
                                            focus:ring-lava/10 w-full resize-none rounded-lg border
                                            px-3 py-2 text-xs text-white placeholder-white/30
                                            transition-all focus:ring-1 focus:outline-none"
                                    />
                                </div>

                                <!-- Visibility -->
                                <div>
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Visibility
                                    </label>
                                    <div class="bg-surface flex gap-0.5 rounded-lg p-0.5">
                                        <button
                                            v-for="opt in visibilityOptions"
                                            :key="opt.key"
                                            type="button"
                                            class="flex flex-1 items-center justify-center gap-1.5
                                                rounded-md py-1.5 text-[11px] font-medium
                                                transition-all"
                                            :class="
                                                visibility === opt.key
                                                    ? 'bg-lava/15 text-lava shadow-sm'
                                                    : 'text-dim hover:text-white'
                                            "
                                            @click="visibility = opt.key"
                                        >
                                            <Icon :name="opt.icon" size="12" />
                                            {{ opt.label }}
                                        </button>
                                    </div>
                                </div>

                                <!-- Tag selection -->
                                <div v-if="availableTags.length > 0">
                                    <label
                                        class="text-dim mb-1.5 block text-[11px] font-medium
                                            tracking-wider uppercase"
                                    >
                                        Tags
                                    </label>
                                    <div
                                        class="flex max-h-24 flex-wrap gap-1.5 overflow-y-auto
                                            py-0.5"
                                    >
                                        <button
                                            v-for="tag in availableTags"
                                            :key="tag.id"
                                            type="button"
                                            class="flex items-center gap-1.5 rounded-full border
                                                px-2.5 py-1 text-[11px] font-medium transition-all"
                                            :class="
                                                selectedTagIds.includes(tag.id)
                                                    ? 'ring-2'
                                                    : 'opacity-60 hover:opacity-100'
                                            "
                                            :style="{
                                                borderColor: tag.color + '60',
                                                backgroundColor: selectedTagIds.includes(tag.id)
                                                    ? tag.color + '20'
                                                    : tag.color + '08',
                                                color: 'white',
                                                '--tw-ring-color': tag.color,
                                            }"
                                            @click="toggleTag(tag.id)"
                                        >
                                            <span
                                                class="inline-block h-2 w-2 shrink-0 rounded-full
                                                    transition-transform"
                                                :class="
                                                    selectedTagIds.includes(tag.id)
                                                        ? 'scale-110'
                                                        : 'scale-100'
                                                "
                                                :style="{ backgroundColor: tag.color }"
                                            />
                                            {{ tag.name }}
                                            <Icon
                                                v-if="selectedTagIds.includes(tag.id)"
                                                name="heroicons:check"
                                                size="10"
                                            />
                                        </button>
                                    </div>
                                </div>

                                <!-- Scene count notice -->
                                <div
                                    v-if="prefillSceneIds?.length"
                                    class="border-border bg-surface/50 flex items-center gap-2
                                        rounded-lg border px-3 py-2"
                                >
                                    <Icon
                                        name="heroicons:film"
                                        size="13"
                                        class="text-dim shrink-0"
                                    />
                                    <span class="text-dim text-[11px]">
                                        {{ prefillSceneIds.length }} scene(s) will be added
                                    </span>
                                </div>
                            </form>
                        </div>

                        <!-- Footer -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-t
                                px-4 py-3"
                        >
                            <span class="text-dim text-[11px]">
                                <template v-if="hasScenes && activeTab === 'existing'">
                                    <span class="text-lava font-medium">
                                        {{ existingPlaylists.length }}
                                    </span>
                                    playlist{{ existingPlaylists.length === 1 ? '' : 's' }}
                                </template>
                                <template v-else-if="selectedTagIds.length > 0">
                                    <span class="text-lava font-medium">
                                        {{ selectedTagIds.length }}
                                    </span>
                                    tag{{ selectedTagIds.length === 1 ? '' : 's' }}
                                </template>
                                <template v-else>
                                    {{ hasScenes && activeTab === 'existing' ? '' : visibility }}
                                </template>
                            </span>
                            <div class="flex items-center gap-2">
                                <button
                                    class="border-border hover:border-border-hover rounded-lg border
                                        px-3 py-1.5 text-xs font-medium text-white transition-all"
                                    @click="emit('close')"
                                >
                                    {{ hasScenes && activeTab === 'existing' ? 'Close' : 'Cancel' }}
                                </button>
                                <button
                                    v-if="!hasScenes || activeTab === 'new'"
                                    :disabled="loading || !name.trim()"
                                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                                        font-semibold text-white transition-colors
                                        disabled:opacity-50"
                                    @click="handleSubmit"
                                >
                                    <span v-if="loading" class="flex items-center gap-1.5">
                                        <Icon name="svg-spinners:90-ring-with-bg" size="12" />
                                        Creating
                                    </span>
                                    <span v-else>Create</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </Transition>
    </Teleport>
</template>
