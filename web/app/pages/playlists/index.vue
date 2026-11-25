<script setup lang="ts">
import { PLAYLIST_SORT_OPTIONS } from '~/types/playlist';
import type {
    PlaylistSortOption,
    PlaylistOwnerFilter,
    PlaylistVisibilityFilter,
    PlaylistKindFilter,
} from '~/types/playlist';

definePageMeta({ middleware: 'auth' });

useHead({ title: 'Playlists' });

useSeoMeta({
    title: 'Playlists',
    ogTitle: 'Playlists - GoonHub',
    description: 'Browse and manage your playlists',
    ogDescription: 'Browse and manage your playlists',
});

const store = usePlaylistStore();
const { showSelector, maxLimit, updatePageSize } = usePageSize();
const route = useRoute();
const patchQuery = useUrlQueryPatch();

const showCreateModal = ref(false);

// Filters, sort and page live in the URL (Back, reload and shared links restore them)
const SORTS = PLAYLIST_SORT_OPTIONS.map((o) => o.value);
const OWNERS: PlaylistOwnerFilter[] = ['me', 'all'];
const VISIBILITIES: PlaylistVisibilityFilter[] = ['', 'public', 'private', 'unlisted'];
const KINDS: PlaylistKindFilter[] = ['', 'scenes', 'markers'];

/** Applies the URL to the store; returns whether the list changed. */
const applyUrl = (): boolean => {
    const q = route.query;
    const next = {
        owner: queryEnum(q.owner, OWNERS, 'me'),
        visibility: queryEnum(q.visibility, VISIBILITIES, ''),
        kind: queryEnum(q.kind, KINDS, ''),
        sort: queryEnum(q.sort, SORTS, 'created_at_desc'),
        tags: queryList(q.tags)
            .map(Number)
            .filter((n) => Number.isInteger(n) && n > 0),
        search: queryString(q.q),
        page: queryPositiveInt(q.page, 1),
    };
    const changed =
        next.owner !== store.ownerFilter ||
        next.visibility !== store.visibilityFilter ||
        next.kind !== store.kindFilter ||
        next.sort !== store.sortOrder ||
        next.tags.join(',') !== store.tagFilter.join(',') ||
        next.search !== store.searchQuery ||
        next.page !== store.currentPage;
    store.ownerFilter = next.owner;
    store.visibilityFilter = next.visibility;
    store.kindFilter = next.kind;
    store.sortOrder = next.sort;
    store.tagFilter = next.tags;
    store.searchQuery = next.search;
    store.currentPage = next.page;
    return changed;
};

// Before the filter watchers below, so that restoring the URL state does not reset the page
let syncingFromUrl = false;
applyUrl();
const searchInput = ref(store.searchQuery);

watch(
    () => [
        store.ownerFilter,
        store.visibilityFilter,
        store.kindFilter,
        store.sortOrder,
        store.tagFilter.join(','),
        store.searchQuery,
        store.currentPage,
    ],
    () =>
        patchQuery({
            owner: unlessDefault(store.ownerFilter, 'me'),
            visibility: store.visibilityFilter || undefined,
            kind: store.kindFilter || undefined,
            sort: unlessDefault(store.sortOrder, 'created_at_desc'),
            tags: store.tagFilter.join(','),
            q: store.searchQuery,
            page: unlessDefault(store.currentPage, 1),
        }),
);

// Links to /playlists while on it (header), browser back/forward
watch(
    () => route.query,
    () => {
        syncingFromUrl = true;
        if (applyUrl()) {
            searchInput.value = store.searchQuery;
            store.loadPlaylists();
        }
        nextTick(() => {
            syncingFromUrl = false;
        });
    },
);

onMounted(async () => {
    await store.loadPlaylists();
});

let searchDebounce: ReturnType<typeof setTimeout> | null = null;

watch(searchInput, (val) => {
    if (searchDebounce) clearTimeout(searchDebounce);
    searchDebounce = setTimeout(() => {
        store.searchQuery = val;
    }, 300);
});

watch(
    [
        () => store.ownerFilter,
        () => store.visibilityFilter,
        () => store.kindFilter,
        () => store.sortOrder,
        () => store.tagFilter,
        () => store.searchQuery,
    ],
    () => {
        if (syncingFromUrl) return;
        store.currentPage = 1;
        store.loadPlaylists();
    },
);

watch(
    () => store.currentPage,
    () => {
        if (syncingFromUrl) return;
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
            <!-- Search -->
            <div class="relative">
                <input
                    v-model="searchInput"
                    type="text"
                    placeholder="Search playlists..."
                    class="border-border bg-surface text-dim w-48 rounded-md border py-1.5 pr-7
                        pl-2.5 text-[11px] transition-all placeholder:text-white/30 focus:text-white
                        focus:outline-none"
                />
                <button
                    v-if="searchInput"
                    class="text-dim absolute top-1/2 right-2 -translate-y-1/2 transition-colors
                        hover:text-white"
                    @click="
                        searchInput = '';
                        store.searchQuery = '';
                    "
                >
                    <Icon name="heroicons:x-mark" size="12" />
                </button>
            </div>

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

            <!-- Kind filter -->
            <div class="flex gap-1">
                <button
                    v-for="opt in [
                        { value: '', label: 'All Kinds', icon: 'heroicons:squares-2x2' },
                        { value: 'scenes', label: 'Scenes', icon: 'heroicons:film' },
                        { value: 'markers', label: 'Markers', icon: 'heroicons:bookmark' },
                    ] as { value: PlaylistKindFilter; label: string; icon: string }[]"
                    :key="opt.value"
                    class="flex items-center gap-1 rounded-md border px-2.5 py-1.5 text-[11px]
                        font-medium transition-all"
                    :class="
                        store.kindFilter === opt.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border bg-surface text-dim hover:text-white'
                    "
                    @click="store.kindFilter = opt.value"
                >
                    <Icon :name="opt.icon" size="12" />
                    {{ opt.label }}
                </button>
            </div>

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
            <Pagination
                v-model="store.currentPage"
                position="top"
                :total="store.total"
                :limit="store.limit"
                :show-page-size-selector="showSelector"
                :max-limit="maxLimit"
                @update:limit="
                    (v: number) => {
                        updatePageSize(v);
                        if (store.currentPage === 1) store.loadPlaylists();
                        else store.currentPage = 1;
                    }
                "
            />

            <PlaylistGrid :playlists="store.playlists" />

            <Pagination
                v-model="store.currentPage"
                :total="store.total"
                :limit="store.limit"
                :show-page-size-selector="showSelector"
                :max-limit="maxLimit"
                @update:limit="
                    (v: number) => {
                        updatePageSize(v);
                        if (store.currentPage === 1) store.loadPlaylists();
                        else store.currentPage = 1;
                    }
                "
            />
        </template>

        <!-- Create Modal -->
        <PlaylistCreateModal
            :visible="showCreateModal"
            @close="showCreateModal = false"
            @created="handleCreated"
        />
    </div>
</template>
