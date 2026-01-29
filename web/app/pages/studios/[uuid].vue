<script setup lang="ts">
import type { Studio, UpdateStudioInput } from '~/types/studio';
import type { Video } from '~/types/video';

const route = useRoute();
const router = useRouter();
const api = useApi();
const authStore = useAuthStore();

const studio = ref<Studio | null>(null);
const videos = ref<Video[]>([]);
const videosTotal = ref(0);
const videosPage = ref(1);
const videosLimit = ref(20);
const isLoading = ref(true);
const isLoadingVideos = ref(false);
const error = ref<string | null>(null);
const showEditModal = ref(false);
const showCreateModal = ref(false);
const showFetchModal = ref(false);

// Rating state
const currentRating = ref(0);
const hoverRating = ref(0);
const isHovering = ref(false);

// Like state
const liked = ref(false);
const likeAnimating = ref(false);

const pageTitle = computed(() => studio.value?.name || 'Studio');
useHead({ title: pageTitle });

const studioUuid = computed(() => route.params.uuid as string);

const isAdmin = computed(() => authStore.user?.role === 'admin');

const displayRating = computed(() => (isHovering.value ? hoverRating.value : currentRating.value));

const loadStudio = async () => {
    try {
        isLoading.value = true;
        error.value = null;
        studio.value = await api.fetchStudioByUUID(studioUuid.value);
        await Promise.all([loadVideos(), loadInteractions()]);
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

const loadVideos = async (page = 1) => {
    try {
        isLoadingVideos.value = true;
        const response = await api.fetchStudioVideos(studioUuid.value, page, videosLimit.value);
        videos.value = response.data;
        videosTotal.value = response.total;
        videosPage.value = page;
    } catch {
        // Ignore video loading errors
    } finally {
        isLoadingVideos.value = false;
    }
};

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
    () => videosPage.value,
    (newPage) => {
        loadVideos(newPage);
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
                @click="goBack"
                class="text-dim hover:text-lava mb-4 flex items-center gap-1 text-sm
                    transition-colors"
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
                                @click="onLikeClick"
                                class="group flex flex-col items-center gap-1 rounded-lg border px-3
                                    py-1.5 transition-all duration-200"
                                :class="[
                                    liked
                                        ? 'border-lava/20 bg-lava/[0.03]'
                                        : `border-border hover:border-border-hover bg-white/[0.02]
                                            hover:bg-white/[0.04]`,
                                ]"
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
                        <div class="flex items-start justify-between">
                            <h1 class="text-2xl font-bold text-white">{{ studio.name }}</h1>
                            <div v-if="isAdmin" class="flex items-center gap-2">
                                <button
                                    @click="showCreateModal = true"
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                >
                                    <Icon name="heroicons:plus" size="14" />
                                    New
                                </button>
                                <button
                                    @click="showFetchModal = true"
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                >
                                    <Icon name="heroicons:cloud-arrow-down" size="14" />
                                    Fetch
                                </button>
                                <button
                                    @click="showEditModal = true"
                                    class="border-border bg-panel hover:border-lava/50
                                        hover:text-lava text-dim flex items-center gap-1 rounded-lg
                                        border px-3 py-1.5 text-sm transition-colors"
                                >
                                    <Icon name="heroicons:pencil" size="14" />
                                    Edit
                                </button>
                            </div>
                        </div>

                        <div v-if="studio.short_name" class="text-dim mt-1 text-sm">
                            {{ studio.short_name }}
                        </div>

                        <div class="text-dim mt-1 text-sm">
                            {{ studio.video_count }}
                            {{ studio.video_count === 1 ? 'video' : 'videos' }}
                        </div>

                        <!-- Quick Info -->
                        <div class="mt-4 space-y-2 text-sm">
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

                        <!-- Description -->
                        <div v-if="studio.description" class="mt-4">
                            <p class="text-dim text-sm leading-relaxed">
                                {{ studio.description }}
                            </p>
                        </div>
                    </div>
                </div>

                <!-- Videos Section -->
                <div>
                    <div class="mb-4 flex items-center justify-between">
                        <h2 class="text-sm font-semibold tracking-wide text-white uppercase">
                            Videos
                        </h2>
                        <span
                            class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                                font-mono text-[11px]"
                        >
                            {{ videosTotal }} videos
                        </span>
                    </div>

                    <div v-if="isLoadingVideos" class="flex h-32 items-center justify-center">
                        <LoadingSpinner label="Loading videos..." />
                    </div>

                    <div
                        v-else-if="videos.length === 0"
                        class="border-border flex h-32 flex-col items-center justify-center
                            rounded-xl border border-dashed text-center"
                    >
                        <Icon name="heroicons:film" size="24" class="text-dim" />
                        <p class="text-dim mt-2 text-sm">No videos found</p>
                    </div>

                    <div v-else>
                        <VideoGrid :videos="videos" />
                        <Pagination
                            v-model="videosPage"
                            :total="videosTotal"
                            :limit="videosLimit"
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
        </div>
    </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}
</style>
