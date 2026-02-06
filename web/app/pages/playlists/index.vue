<script setup lang="ts">
import { PLAYLIST_SORT_OPTIONS } from '~/types/playlist';
import type {
    PlaylistSortOption,
    PlaylistOwnerFilter,
    PlaylistVisibilityFilter,
} from '~/types/playlist';

definePageMeta({ middleware: 'auth' });

useHead({ title: 'Playlists' });

const store = usePlaylistStore();

const showCreateModal = ref(false);

onMounted(async () => {
    await store.loadPlaylists();
});

watch(
    [
        () => store.ownerFilter,
        () => store.visibilityFilter,
        () => store.sortOrder,
        () => store.tagFilter,
    ],
    () => {
        store.currentPage = 1;
        store.loadPlaylists();
    },
);

watch(
    () => store.currentPage,
    () => {
        store.loadPlaylists();
    },
);

const handleCreated = () => {
    showCreateModal.value = false;
    store.loadPlaylists();
};
</script>

<template>
    <div class="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        <!-- Header -->
        <div class="mb-6 flex items-center justify-between">
            <div>
                <h1 class="text-lg font-semibold text-white">Playlists</h1>
                <p class="text-dim mt-0.5 text-xs">
                    {{ store.total }} playlist{{ store.total !== 1 ? 's' : '' }}
                </p>
            </div>
            <button
                class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4 py-2
                    text-xs font-semibold text-white transition-all"
                @click="showCreateModal = true"
            >
                <Icon name="heroicons:plus" size="16" />
                Create
            </button>
        </div>

        <!-- Filters -->
        <div class="mb-6 flex flex-wrap items-center gap-3">
            <!-- Owner filter -->
            <div class="flex gap-1">
                <button
                    v-for="opt in [
                        { value: 'me', label: 'My Playlists' },
                        { value: 'all', label: 'All' },
                    ] as { value: PlaylistOwnerFilter; label: string }[]"
                    :key="opt.value"
                    class="rounded-md border px-3 py-1.5 text-[11px] font-medium transition-all"
                    :class="
                        store.ownerFilter === opt.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border bg-surface text-dim hover:text-white'
                    "
                    @click="store.ownerFilter = opt.value"
                >
                    {{ opt.label }}
                </button>
            </div>

            <!-- Visibility filter -->
            <select
                :value="store.visibilityFilter"
                class="border-border bg-surface text-dim rounded-md border px-2 py-1.5 text-[11px]
                    transition-all focus:outline-none"
                @change="
                    store.visibilityFilter = ($event.target as HTMLSelectElement)
                        .value as PlaylistVisibilityFilter
                "
            >
                <option value="">All Visibility</option>
                <option value="public">Public</option>
                <option value="private">Private</option>
            </select>

            <!-- Sort -->
            <select
                :value="store.sortOrder"
                class="border-border bg-surface text-dim rounded-md border px-2 py-1.5 text-[11px]
                    transition-all focus:outline-none"
                @change="
                    store.sortOrder = ($event.target as HTMLSelectElement)
                        .value as PlaylistSortOption
                "
            >
                <option v-for="opt in PLAYLIST_SORT_OPTIONS" :key="opt.value" :value="opt.value">
                    {{ opt.label }}
                </option>
            </select>
        </div>

        <!-- Error -->
        <ErrorAlert v-if="store.error" :message="store.error" class="mb-4" />

        <!-- Loading -->
        <div v-if="store.isLoading" class="flex justify-center py-16">
            <LoadingSpinner />
        </div>

        <!-- Empty -->
        <div
            v-else-if="store.playlists.length === 0"
            class="border-border flex flex-col items-center justify-center rounded-lg border
                border-dashed py-16"
        >
            <Icon name="heroicons:queue-list" size="40" class="text-dim mb-3" />
            <p class="text-dim text-sm">No playlists found</p>
            <button
                class="text-lava hover:text-lava-glow mt-3 text-xs font-medium transition-colors"
                @click="showCreateModal = true"
            >
                Create your first playlist
            </button>
        </div>

        <!-- Grid -->
        <template v-else>
            <PlaylistGrid :playlists="store.playlists" />

            <Pagination v-model="store.currentPage" :total="store.total" :limit="store.limit" />
        </template>

        <!-- Create Modal -->
        <PlaylistCreateModal
            :visible="showCreateModal"
            @close="showCreateModal = false"
            @created="handleCreated"
        />
    </div>
</template>
