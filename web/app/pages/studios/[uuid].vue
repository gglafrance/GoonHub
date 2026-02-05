<script setup lang="ts">
import type { Studio, UpdateStudioInput } from '~/types/studio';
import type { Scene } from '~/types/scene';

const route = useRoute();
const router = useRouter();
const api = useApi();
const authStore = useAuthStore();
const settingsStore = useSettingsStore();

const studio = ref<Studio | null>(null);
const scenes = ref<Scene[]>([]);
const scenesTotal = ref(0);
const scenesPage = useUrlPagination();
const scenesLimit = computed(() => settingsStore.videosPerPage);
const isLoading = ref(true);
const isLoadingScenes = ref(false);
const error = ref<string | null>(null);
const showEditModal = ref(false);
const showCreateModal = ref(false);
const showFetchModal = ref(false);
const showDeleteModal = ref(false);

// Scene search/sort state
const scenesQuery = ref('');
const defaultSort = settingsStore.sortPreferences?.studio_scenes ?? '';
const scenesSort = ref(
    typeof route.query.sort === 'string' && route.query.sort ? route.query.sort : defaultSort,
);
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

const sortOptions = [
    { value: '', label: 'Newest' },
    { value: 'random', label: 'Random' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
];

// Random sort seed (synced to URL for persistence across refresh)
const generateSeed = () => Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
const scenesSeed = ref(route.query.seed ? Number(route.query.seed) : 0);

// Single URL sync for sort + seed to avoid race conditions
const syncSortToUrl = () => {
    const query = { ...route.query };
    if (scenesSort.value === defaultSort || !scenesSort.value) {
        delete query.sort;
    } else {
        query.sort = scenesSort.value;
    }
    if (scenesSort.value === 'random' && scenesSeed.value) {
        query.seed = String(scenesSeed.value);
    } else {
        delete query.seed;
    }
    router.replace({ query });
};

const reshuffle = () => {
    scenesSeed.value = generateSeed();
    syncSortToUrl();
    scenesPage.value = 1;
    loadScenes(1);
};

// Rating state
const currentRating = ref(0);
const hoverRating = ref(0);
const isHovering = ref(false);

// Like state
const liked = ref(false);
const likeAnimating = ref(false);

// Mobile admin menu
const showAdminMenu = ref(false);

// Details collapsed state (collapsed by default on mobile)
const detailsExpanded = ref(false);

const pageTitle = computed(() => studio.value?.name || 'Studio');
useHead({ title: pageTitle });

// Dynamic OG metadata
watch(
    studio,
    (s) => {
        if (s) {
            useSeoMeta({
                title: s.name,
                ogTitle: s.name,
                description: s.description || `${s.name} - ${s.scene_count} scenes on GoonHub`,
                ogDescription: s.description || `${s.name} - ${s.scene_count} scenes on GoonHub`,
                ogImage: s.logo || undefined,
                ogType: 'website',
            });
        }
    },
    { immediate: true },
);

const studioUuid = computed(() => route.params.uuid as string);

const isAdmin = computed(() => authStore.user?.role === 'admin');

const displayRating = computed(() => (isHovering.value ? hoverRating.value : currentRating.value));

const loadStudio = async () => {
    try {
        isLoading.value = true;
        error.value = null;
        studio.value = await api.fetchStudioByUUID(studioUuid.value);
        await Promise.all([loadScenes(scenesPage.value), loadInteractions()]);
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load studio';
    } finally {
        isLoading.value = false;
    }
};

const loadInteractions = async () => {
    if (!studio.value) return;
    try {
        const res = await api.fetchStudioInteractions(studio.value.uuid);
        currentRating.value = res.rating || 0;
        liked.value = res.liked || false;
    } catch {
        // Silently fail for interactions
    }
};

function getStarState(starIndex: number): 'full' | 'half' | 'empty' {
    const rating = displayRating.value;
    if (rating >= starIndex) return 'full';
    if (rating >= starIndex - 0.5) return 'half';
    return 'empty';
}

function onStarHover(starIndex: number, isLeftHalf: boolean) {
    isHovering.value = true;
    hoverRating.value = isLeftHalf ? starIndex - 0.5 : starIndex;
}

function onStarLeave() {
    isHovering.value = false;
    hoverRating.value = 0;
}

async function onStarClick(starIndex: number, isLeftHalf: boolean) {
    if (!studio.value) return;
    const newRating = isLeftHalf ? starIndex - 0.5 : starIndex;

    if (newRating === currentRating.value) {
        // Clicking same rating clears it
        currentRating.value = 0;
        try {
            await api.deleteStudioRating(studio.value.uuid);
        } catch {
            currentRating.value = newRating;
        }
    } else {
        const oldRating = currentRating.value;
        currentRating.value = newRating;
        try {
            await api.setStudioRating(studio.value.uuid, newRating);
        } catch {
            currentRating.value = oldRating;
        }
    }
}

async function onLikeClick() {
    if (!studio.value) return;
    const wasLiked = liked.value;
    liked.value = !wasLiked;
    likeAnimating.value = true;
    setTimeout(() => {
        likeAnimating.value = false;
    }, 300);

    try {
        const res = await api.toggleStudioLike(studio.value.uuid);
        liked.value = res.liked;
    } catch {
        liked.value = wasLiked;
    }
}

const loadScenes = async (page: number) => {
    if (!studio.value) return;
    try {
        isLoadingScenes.value = true;
        const response = await api.fetchStudioScenes(
            studioUuid.value,
            page,
            scenesLimit.value,
            scenesQuery.value || undefined,
            scenesSort.value || undefined,
            studio.value.name,
            scenesSort.value === 'random' ? scenesSeed.value : undefined,
        );
        scenes.value = response.data;
        scenesTotal.value = response.total;
    } catch {
        // Ignore scene loading errors
    } finally {
        isLoadingScenes.value = false;
    }
};

const onSearchInput = () => {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
        scenesPage.value = 1;
        loadScenes(1);
    }, 300);
};

watch(scenesSort, (newSort) => {
    if (newSort === 'random' && scenesSeed.value === 0) {
        scenesSeed.value = generateSeed();
    } else if (newSort !== 'random') {
        scenesSeed.value = 0;
    }
    syncSortToUrl();
    scenesPage.value = 1;
    loadScenes(1);
});

// Handle browser back/forward navigation
watch(
    () => route.query.sort,
    () => {
        const urlSort =
            typeof route.query.sort === 'string' && route.query.sort
                ? route.query.sort
                : defaultSort;
        if (scenesSort.value !== urlSort) {
            scenesSort.value = urlSort;
        }
    },
);

onMounted(() => {
    loadStudio();
});

watch(
    () => route.params.uuid,
    () => {
        loadStudio();
    },
);

watch(
    () => scenesPage.value,
    (newPage) => {
        loadScenes(newPage);
    },
);

const goBack = () => {
    router.push('/studios');
};

const handleStudioUpdated = () => {
    showEditModal.value = false;
    loadStudio();
};

const handleStudioCreated = (newStudio: Studio) => {
    showCreateModal.value = false;
    router.push(`/studios/${newStudio.uuid}`);
};

const handleStudioDeleted = () => {
    showDeleteModal.value = false;
    router.push('/studios');
};

const handleApplyMetadata = async (data: Partial<UpdateStudioInput>) => {
    if (!studio.value) return;
    try {
        await api.updateStudio(studio.value.id, data);
        showFetchModal.value = false;
        await loadStudio();
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update studio';
    }
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Back Button -->
            <button
                class="text-dim hover:text-lava mb-4 flex items-center gap-1 text-sm
                    transition-colors"
                @click="goBack"
            >
                <Icon name="heroicons:arrow-left" size="16" />
                Back to Studios
            </button>

            <!-- Loading State -->
            <div v-if="isLoading" class="flex h-64 items-center justify-center">
                <LoadingSpinner label="Loading studio..." />
            </div>

            <!-- Error State -->
            <ErrorAlert v-else-if="error" :message="error" />

            <!-- Studio Details -->
            <div v-else-if="studio" class="space-y-6">
                <!-- Header Section -->
                <div class="flex flex-col gap-6 sm:flex-row">
                    <!-- Studio Logo + Rating -->
                    <div class="flex shrink-0 flex-col items-center gap-3">
                        <div
                            class="bg-surface border-border h-48 w-48 overflow-hidden rounded-lg
                                border"
                        >
                            <img
                                v-if="studio.logo"
                                :src="studio.logo"
                                :alt="studio.name"
                                class="h-full w-full object-contain p-4"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:building-office-2" size="64" />
                            </div>
                        </div>

                        <!-- Rating & Like -->
                        <div class="flex items-center gap-3">
                            <!-- Rating -->
                            <div class="flex flex-col items-center gap-1">
                                <div class="flex items-center gap-[3px]" @mouseleave="onStarLeave">
                                    <div
                                        v-for="star in 5"
                                        :key="star"
                                        class="relative h-[18px] w-[18px] cursor-pointer"
                                    >
                                        <div
                                            class="absolute inset-y-0 left-0 z-10 w-1/2"
                                            @mouseenter="onStarHover(star, true)"
                                            @click="onStarClick(star, true)"
                                        />
                                        <div
                                            class="absolute inset-y-0 right-0 z-10 w-1/2"
                                            @mouseenter="onStarHover(star, false)"
                                            @click="onStarClick(star, false)"
                                        />

                                        <Icon
                                            name="heroicons:star"
                                            size="18"
                                            class="absolute inset-0 transition-all duration-150"
                                            :class="[
                                                isHovering ? 'text-white/30' : 'text-white/15',
                                            ]"
                                        />

                                        <Icon
                                            v-if="getStarState(star) === 'full'"
                                            name="heroicons:star-solid"
                                            size="18"
                                            class="absolute inset-0 transition-all duration-150"
                                            :class="[isHovering ? 'text-lava-glow' : 'text-lava']"
                                        />

                                        <div
                                            v-if="getStarState(star) === 'half'"
                                            class="absolute inset-0 overflow-hidden"
                                            style="width: 50%"
                                        >
                                            <Icon
                                                name="heroicons:star-solid"
                                                size="18"
                                                class="transition-all duration-150"
                                                :class="[
                                                    isHovering ? 'text-lava-glow' : 'text-lava',
                                                ]"
                                            />
                                        </div>
                                    </div>
                                </div>
                                <Transition name="fade" mode="out-in">
                                    <span
                                        v-if="displayRating > 0"
                                        :key="displayRating"
                                        class="text-[11px] font-medium tabular-nums"
                                        :class="[isHovering ? 'text-white/50' : 'text-lava/70']"
                                    >
                                        {{ displayRating.toFixed(1) }}
                                    </span>
                                    <span v-else class="text-[10px] text-white/25">Rate</span>
                                </Transition>
                            </div>

                            <!-- Like button -->
                            <button
                                class="group flex flex-col items-center gap-1 rounded-lg border px-3
                                    py-1.5 transition-all duration-200"
                                :class="[
                                    liked
                                        ? 'border-lava/20 bg-lava/[0.03]'
                                        : `border-border hover:border-border-hover bg-white/[0.02]
                                            hover:bg-white/[0.04]`,
                                ]"
                                @click="onLikeClick"
                            >
                                <div
                                    class="transition-all duration-200"
                                    :class="[
                                        liked
                                            ? 'text-lava'
                                            : 'text-white/20 group-hover:text-white/40',
                                        likeAnimating ? 'scale-125' : 'scale-100',
                                    ]"
                                >
                                    <Icon
                                        :name="liked ? 'heroicons:heart-solid' : 'heroicons:heart'"
                                        size="16"
                                    />
                                </div>
                                <span
                                    class="text-[10px] font-medium transition-colors duration-200"
                                    :class="[
                                        liked
                                            ? 'text-lava/60'
                                            : 'text-white/25 group-hover:text-white/40',
                                    ]"
                                >
                                    {{ liked ? 'Liked' : 'Like' }}
                                </span>
                            </button>
                        </div>
                    </div>

                    <!-- Basic Info -->
                    <div class="flex-1">
                        <div class="flex items-start justify-between gap-3">
                            <div class="min-w-0 flex-1">
                                <h1 class="truncate text-xl font-bold text-white sm:text-2xl">
                                    {{ studio.name }}
                                </h1>
                                <div v-if="studio.short_name" class="text-dim mt-1 text-sm">
                                    {{ studio.short_name }}
                                </div>
                                <div class="text-dim mt-1 text-sm">
                                    {{ studio.scene_count }}
                                    {{ studio.scene_count === 1 ? 'scene' : 'scenes' }}
                                </div>
                            </div>

                            <!-- Desktop: Full admin buttons -->
                            <div v-if="isAdmin" class="hidden items-center gap-2 sm:flex">
                                <button
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                    @click="showCreateModal = true"
                                >
                                    <Icon name="heroicons:plus" size="14" />
                                    New
                                </button>
                                <button
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                    @click="showFetchModal = true"
                                >
                                    <Icon name="heroicons:cloud-arrow-down" size="14" />
                                    Fetch
                                </button>
                                <button
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                    @click="showEditModal = true"
                                >
                                    <Icon name="heroicons:pencil" size="14" />
                                    Edit
                                </button>
                                <button
                                    class="border-border bg-panel text-dim flex items-center gap-1
                                        rounded-lg border px-3 py-1.5 text-sm transition-colors
                                        hover:border-red-500/50 hover:text-red-500"
                                    @click="showDeleteModal = true"
                                >
                                    <Icon name="heroicons:trash" size="14" />
                                    Delete
                                </button>
                            </div>

                            <!-- Mobile: Dropdown menu -->
                            <div v-if="isAdmin" class="relative sm:hidden">
                                <button
                                    class="border-border bg-panel text-dim flex h-9 w-9 items-center
                                        justify-center rounded-lg border transition-colors
                                        hover:border-white/20 hover:text-white"
                                    @click="showAdminMenu = !showAdminMenu"
                                >
                                    <Icon name="heroicons:ellipsis-vertical" size="18" />
                                </button>

                                <!-- Dropdown -->
                                <Transition name="fade">
                                    <div
                                        v-if="showAdminMenu"
                                        class="border-border bg-surface absolute top-11 right-0 z-20
                                            w-40 rounded-lg border py-1 shadow-xl"
                                    >
                                        <button
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                            @click="
                                                showCreateModal = true;
                                                showAdminMenu = false;
                                            "
                                        >
                                            <Icon name="heroicons:plus" size="16" />
                                            New Studio
                                        </button>
                                        <button
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                            @click="
                                                showFetchModal = true;
                                                showAdminMenu = false;
                                            "
                                        >
                                            <Icon name="heroicons:cloud-arrow-down" size="16" />
                                            Fetch Metadata
                                        </button>
                                        <button
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                            @click="
                                                showEditModal = true;
                                                showAdminMenu = false;
                                            "
                                        >
                                            <Icon name="heroicons:pencil" size="16" />
                                            Edit
                                        </button>
                                        <div class="border-border my-1 border-t" />
                                        <button
                                            class="flex w-full items-center gap-2 px-3 py-2 text-sm
                                                text-red-400 transition-colors hover:bg-red-500/10"
                                            @click="
                                                showDeleteModal = true;
                                                showAdminMenu = false;
                                            "
                                        >
                                            <Icon name="heroicons:trash" size="16" />
                                            Delete
                                        </button>
                                    </div>
                                </Transition>

                                <!-- Backdrop to close menu -->
                                <div
                                    v-if="showAdminMenu"
                                    class="fixed inset-0 z-10"
                                    @click="showAdminMenu = false"
                                />
                            </div>
                        </div>

                        <!-- Quick Info - Desktop only -->
                        <div class="mt-4 hidden space-y-2 text-sm sm:block">
                            <div v-if="studio.url">
                                <span class="text-dim">Website:</span>
                                <a
                                    :href="studio.url"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="text-lava hover:text-lava-glow ml-1 transition-colors"
                                >
                                    {{ studio.url }}
                                </a>
                            </div>
                            <div v-if="studio.rating">
                                <span class="text-dim">PornDB Rating:</span>
                                <span class="ml-1 text-white">{{ studio.rating }}</span>
                            </div>
                        </div>

                        <!-- Description - Desktop only -->
                        <div v-if="studio.description" class="mt-4 hidden sm:block">
                            <p class="text-dim text-sm leading-relaxed">
                                {{ studio.description }}
                            </p>
                        </div>
                    </div>
                </div>

                <!-- Mobile: Collapsible Details Section -->
                <div v-if="studio.url || studio.rating || studio.description" class="sm:hidden">
                    <button
                        class="border-border bg-surface hover:bg-elevated flex w-full items-center
                            justify-between rounded-lg border px-4 py-3 transition-colors"
                        @click="detailsExpanded = !detailsExpanded"
                    >
                        <span class="text-sm font-medium text-white">Studio Details</span>
                        <Icon
                            name="heroicons:chevron-down"
                            size="18"
                            class="text-dim transition-transform duration-200"
                            :class="{ 'rotate-180': detailsExpanded }"
                        />
                    </button>

                    <Transition name="collapse">
                        <div v-if="detailsExpanded" class="mt-3 space-y-3">
                            <div class="border-border bg-surface rounded-lg border p-4">
                                <dl class="space-y-3 text-sm">
                                    <div v-if="studio.url">
                                        <dt class="text-dim text-[11px] uppercase">Website</dt>
                                        <dd>
                                            <a
                                                :href="studio.url"
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                class="text-lava hover:text-lava-glow break-all
                                                    transition-colors"
                                            >
                                                {{ studio.url }}
                                            </a>
                                        </dd>
                                    </div>
                                    <div v-if="studio.rating">
                                        <dt class="text-dim text-[11px] uppercase">
                                            PornDB Rating
                                        </dt>
                                        <dd class="text-white">{{ studio.rating }}</dd>
                                    </div>
                                    <div v-if="studio.description">
                                        <dt class="text-dim text-[11px] uppercase">Description</dt>
                                        <dd class="text-dim mt-1 leading-relaxed">
                                            {{ studio.description }}
                                        </dd>
                                    </div>
                                </dl>
                            </div>
                        </div>
                    </Transition>
                </div>

                <!-- Scenes Section -->
                <div>
                    <div class="mb-4 flex items-center justify-between">
                        <h2 class="text-sm font-semibold tracking-wide text-white uppercase">
                            Scenes
                        </h2>
                        <span
                            class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                                font-mono text-[11px]"
                        >
                            {{ scenesTotal }} scenes
                        </span>
                    </div>

                    <!-- Search and Sort Controls -->
                    <div class="mb-4 flex flex-row gap-2 sm:items-center sm:gap-3">
                        <!-- Search Input -->
                        <div class="relative flex-1">
                            <Icon
                                name="heroicons:magnifying-glass"
                                size="16"
                                class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                            />
                            <input
                                v-model="scenesQuery"
                                type="text"
                                placeholder="Search scenes..."
                                class="border-border bg-surface placeholder:text-dim h-10 w-full
                                    rounded-lg border py-2 pr-3 pl-9 text-sm text-white
                                    transition-colors focus:border-white/20 focus:outline-none"
                                @input="onSearchInput"
                            />
                        </div>

                        <!-- Sort Dropdown -->
                        <UiSortSelect v-model="scenesSort" :options="sortOptions" />

                        <button
                            v-if="scenesSort === 'random'"
                            class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10
                                flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
                                border transition-all"
                            title="Reshuffle"
                            @click="reshuffle()"
                        >
                            <Icon name="heroicons:arrow-path" size="16" class="text-white" />
                        </button>
                    </div>

                    <div v-if="isLoadingScenes" class="flex h-32 items-center justify-center">
                        <LoadingSpinner label="Loading scenes..." />
                    </div>

                    <div
                        v-else-if="scenes.length === 0"
                        class="border-border flex h-32 flex-col items-center justify-center
                            rounded-xl border border-dashed text-center"
                    >
                        <Icon name="heroicons:film" size="24" class="text-dim" />
                        <p class="text-dim mt-2 text-sm">No scenes found</p>
                    </div>

                    <div v-else>
                        <SceneGrid :scenes="scenes" />
                        <Pagination
                            v-model="scenesPage"
                            :total="scenesTotal"
                            :limit="scenesLimit"
                        />
                    </div>
                </div>
            </div>

            <!-- Edit Modal -->
            <StudiosEditModal
                v-if="studio && showEditModal"
                :studio="studio"
                :visible="showEditModal"
                @close="showEditModal = false"
                @updated="handleStudioUpdated"
            />

            <!-- Create Modal -->
            <StudiosEditModal
                v-if="showCreateModal"
                :studio="null"
                :visible="showCreateModal"
                @close="showCreateModal = false"
                @created="handleStudioCreated"
            />

            <!-- Fetch Metadata Modal -->
            <StudiosFetchMetadataModal
                v-if="showFetchModal && studio"
                :visible="showFetchModal"
                :studio-name="studio.name"
                :current-studio="studio"
                @close="showFetchModal = false"
                @apply="handleApplyMetadata"
            />

            <!-- Delete Modal -->
            <StudiosDeleteModal
                v-if="studio && showDeleteModal"
                :visible="showDeleteModal"
                :studio="studio"
                @close="showDeleteModal = false"
                @deleted="handleStudioDeleted"
            />
        </div>
    </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}

.collapse-enter-active,
.collapse-leave-active {
    transition: all 0.2s ease;
    overflow: hidden;
}
.collapse-enter-from,
.collapse-leave-to {
    opacity: 0;
    transform: translateY(-8px);
}
</style>
