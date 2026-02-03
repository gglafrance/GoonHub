<script setup lang="ts">
import type { Actor, UpdateActorInput } from '~/types/actor';
import type { Scene } from '~/types/scene';

const route = useRoute();
const router = useRouter();
const api = useApi();
const authStore = useAuthStore();
const settingsStore = useSettingsStore();

const actor = ref<Actor | null>(null);
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
const scenesSort = ref('');
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

const sortOptions = [
    { value: '', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
];

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

const pageTitle = computed(() => actor.value?.name || 'Actor');
useHead({ title: pageTitle });

// Dynamic OG metadata
watch(
    actor,
    (a) => {
        if (a) {
            useSeoMeta({
                title: a.name,
                ogTitle: a.name,
                description: `${a.name} - ${a.scene_count} scenes on GoonHub`,
                ogDescription: `${a.name} - ${a.scene_count} scenes on GoonHub`,
                ogImage: a.image_url || undefined,
                ogType: 'profile',
            });
        }
    },
    { immediate: true },
);

const actorUuid = computed(() => route.params.uuid as string);

const isAdmin = computed(() => authStore.user?.role === 'admin');

const displayRating = computed(() => (isHovering.value ? hoverRating.value : currentRating.value));

const loadActor = async () => {
    try {
        isLoading.value = true;
        error.value = null;
        actor.value = await api.fetchActorByUUID(actorUuid.value);
        await Promise.all([loadScenes(scenesPage.value), loadInteractions()]);
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load actor';
    } finally {
        isLoading.value = false;
    }
};

const loadInteractions = async () => {
    if (!actor.value) return;
    try {
        const res = await api.fetchActorInteractions(actor.value.uuid);
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
    if (!actor.value) return;
    const newRating = isLeftHalf ? starIndex - 0.5 : starIndex;

    if (newRating === currentRating.value) {
        // Clicking same rating clears it
        currentRating.value = 0;
        try {
            await api.deleteActorRating(actor.value.uuid);
        } catch {
            currentRating.value = newRating;
        }
    } else {
        const oldRating = currentRating.value;
        currentRating.value = newRating;
        try {
            await api.setActorRating(actor.value.uuid, newRating);
        } catch {
            currentRating.value = oldRating;
        }
    }
}

async function onLikeClick() {
    if (!actor.value) return;
    const wasLiked = liked.value;
    liked.value = !wasLiked;
    likeAnimating.value = true;
    setTimeout(() => {
        likeAnimating.value = false;
    }, 300);

    try {
        const res = await api.toggleActorLike(actor.value.uuid);
        liked.value = res.liked;
    } catch {
        liked.value = wasLiked;
    }
}

const loadScenes = async (page: number) => {
    if (!actor.value) return;
    try {
        isLoadingScenes.value = true;
        const response = await api.fetchActorScenes(
            actorUuid.value,
            page,
            scenesLimit.value,
            scenesQuery.value || undefined,
            scenesSort.value || undefined,
            actor.value.name,
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

const onSortChange = () => {
    scenesPage.value = 1;
    loadScenes(1);
};

onMounted(() => {
    loadActor();
});

watch(
    () => route.params.uuid,
    () => {
        loadActor();
    },
);

watch(
    () => scenesPage.value,
    (newPage) => {
        loadScenes(newPage);
    },
);

const goBack = () => {
    router.push('/actors');
};

const handleActorUpdated = () => {
    showEditModal.value = false;
    loadActor();
};

const handleActorCreated = (newActor: Actor) => {
    showCreateModal.value = false;
    router.push(`/actors/${newActor.uuid}`);
};

const handleActorDeleted = () => {
    showDeleteModal.value = false;
    router.push('/actors');
};

const handleApplyMetadata = async (data: Partial<UpdateActorInput>) => {
    if (!actor.value) return;
    try {
        await api.updateActor(actor.value.id, data);
        showFetchModal.value = false;
        await loadActor();
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update actor';
    }
};

const formatAge = (birthday: string) => {
    const birthDate = new Date(birthday);
    const today = new Date();
    let age = today.getFullYear() - birthDate.getFullYear();
    const monthDiff = today.getMonth() - birthDate.getMonth();
    if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDate.getDate())) {
        age--;
    }
    return age;
};

const formatHeight = (cm: number) => {
    const feet = Math.floor(cm / 30.48);
    const inches = Math.round((cm % 30.48) / 2.54);
    return `${feet}'${inches}" (${cm}cm)`;
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
                Back to Actors
            </button>

            <!-- Loading State -->
            <div v-if="isLoading" class="flex h-64 items-center justify-center">
                <LoadingSpinner label="Loading actor..." />
            </div>

            <!-- Error State -->
            <ErrorAlert v-else-if="error" :message="error" />

            <!-- Actor Details -->
            <div v-else-if="actor" class="space-y-6">
                <!-- Header Section -->
                <div class="flex flex-col gap-6 sm:flex-row">
                    <!-- Actor Image + Rating -->
                    <div class="flex shrink-0 flex-col items-center gap-3">
                        <div
                            class="bg-surface border-border h-64 w-48 overflow-hidden rounded-lg
                                border"
                        >
                            <img
                                v-if="actor.image_url"
                                :src="actor.image_url"
                                :alt="actor.name"
                                class="h-full w-full object-cover"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:user" size="64" />
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
                        <div class="flex items-start justify-between gap-3">
                            <div class="min-w-0 flex-1">
                                <h1 class="truncate text-xl font-bold text-white sm:text-2xl">
                                    {{ actor.name }}
                                </h1>
                                <div class="text-dim mt-1 text-sm">
                                    {{ actor.scene_count }}
                                    {{ actor.scene_count === 1 ? 'scene' : 'scenes' }}
                                </div>
                            </div>

                            <!-- Desktop: Full admin buttons -->
                            <div v-if="isAdmin" class="hidden items-center gap-2 sm:flex">
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
                                <button
                                    @click="showDeleteModal = true"
                                    class="border-border bg-panel text-dim flex items-center gap-1
                                        rounded-lg border px-3 py-1.5 text-sm transition-colors
                                        hover:border-red-500/50 hover:text-red-500"
                                >
                                    <Icon name="heroicons:trash" size="14" />
                                    Delete
                                </button>
                            </div>

                            <!-- Mobile: Dropdown menu -->
                            <div v-if="isAdmin" class="relative sm:hidden">
                                <button
                                    @click="showAdminMenu = !showAdminMenu"
                                    class="border-border bg-panel text-dim flex h-9 w-9 items-center
                                        justify-center rounded-lg border transition-colors
                                        hover:border-white/20 hover:text-white"
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
                                            @click="
                                                showCreateModal = true;
                                                showAdminMenu = false;
                                            "
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                        >
                                            <Icon name="heroicons:plus" size="16" />
                                            New Actor
                                        </button>
                                        <button
                                            @click="
                                                showFetchModal = true;
                                                showAdminMenu = false;
                                            "
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                        >
                                            <Icon name="heroicons:cloud-arrow-down" size="16" />
                                            Fetch Metadata
                                        </button>
                                        <button
                                            @click="
                                                showEditModal = true;
                                                showAdminMenu = false;
                                            "
                                            class="text-dim flex w-full items-center gap-2 px-3 py-2
                                                text-sm transition-colors hover:bg-white/5
                                                hover:text-white"
                                        >
                                            <Icon name="heroicons:pencil" size="16" />
                                            Edit
                                        </button>
                                        <div class="border-border my-1 border-t" />
                                        <button
                                            @click="
                                                showDeleteModal = true;
                                                showAdminMenu = false;
                                            "
                                            class="flex w-full items-center gap-2 px-3 py-2 text-sm
                                                text-red-400 transition-colors hover:bg-red-500/10"
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
                        <div class="mt-4 hidden grid-cols-2 gap-3 text-sm sm:grid">
                            <div v-if="actor.nationality">
                                <span class="text-dim">Nationality:</span>
                                <span class="ml-1 text-white">{{ actor.nationality }}</span>
                            </div>
                            <div v-if="actor.birthday">
                                <span class="text-dim">Age:</span>
                                <span class="ml-1 text-white">{{ formatAge(actor.birthday) }}</span>
                            </div>
                            <div v-if="actor.height_cm">
                                <span class="text-dim">Height:</span>
                                <span class="ml-1 text-white">{{
                                    formatHeight(actor.height_cm)
                                }}</span>
                            </div>
                            <div v-if="actor.measurements">
                                <span class="text-dim">Measurements:</span>
                                <span class="ml-1 text-white">{{ actor.measurements }}</span>
                            </div>
                            <div v-if="actor.hair_color">
                                <span class="text-dim">Hair:</span>
                                <span class="ml-1 text-white">{{ actor.hair_color }}</span>
                            </div>
                            <div v-if="actor.eye_color">
                                <span class="text-dim">Eyes:</span>
                                <span class="ml-1 text-white">{{ actor.eye_color }}</span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Mobile: Collapsible Details Section -->
                <div class="sm:hidden">
                    <button
                        @click="detailsExpanded = !detailsExpanded"
                        class="border-border bg-surface hover:bg-elevated flex w-full items-center
                            justify-between rounded-lg border px-4 py-3 transition-colors"
                    >
                        <span class="text-sm font-medium text-white">Actor Details</span>
                        <Icon
                            name="heroicons:chevron-down"
                            size="18"
                            class="text-dim transition-transform duration-200"
                            :class="{ 'rotate-180': detailsExpanded }"
                        />
                    </button>

                    <Transition name="collapse">
                        <div v-if="detailsExpanded" class="mt-3 space-y-3">
                            <!-- Quick Info (mobile) -->
                            <div class="border-border bg-surface rounded-lg border p-4">
                                <h3 class="mb-3 text-xs font-semibold text-white uppercase">
                                    Quick Info
                                </h3>
                                <dl class="grid grid-cols-2 gap-2 text-sm">
                                    <div v-if="actor.nationality">
                                        <dt class="text-dim text-[11px]">Nationality</dt>
                                        <dd class="text-white">{{ actor.nationality }}</dd>
                                    </div>
                                    <div v-if="actor.birthday">
                                        <dt class="text-dim text-[11px]">Age</dt>
                                        <dd class="text-white">{{ formatAge(actor.birthday) }}</dd>
                                    </div>
                                    <div v-if="actor.height_cm">
                                        <dt class="text-dim text-[11px]">Height</dt>
                                        <dd class="text-white">
                                            {{ formatHeight(actor.height_cm) }}
                                        </dd>
                                    </div>
                                    <div v-if="actor.measurements">
                                        <dt class="text-dim text-[11px]">Measurements</dt>
                                        <dd class="text-white">{{ actor.measurements }}</dd>
                                    </div>
                                    <div v-if="actor.hair_color">
                                        <dt class="text-dim text-[11px]">Hair</dt>
                                        <dd class="text-white">{{ actor.hair_color }}</dd>
                                    </div>
                                    <div v-if="actor.eye_color">
                                        <dt class="text-dim text-[11px]">Eyes</dt>
                                        <dd class="text-white">{{ actor.eye_color }}</dd>
                                    </div>
                                </dl>
                            </div>

                            <!-- Demographics (mobile) -->
                            <div
                                v-if="
                                    actor.birthplace ||
                                    actor.ethnicity ||
                                    actor.astrology ||
                                    actor.birthday
                                "
                                class="border-border bg-surface rounded-lg border p-4"
                            >
                                <h3 class="mb-3 text-xs font-semibold text-white uppercase">
                                    Demographics
                                </h3>
                                <dl class="space-y-2 text-sm">
                                    <div v-if="actor.birthday" class="flex justify-between">
                                        <dt class="text-dim">Birthday</dt>
                                        <dd class="text-white">
                                            {{ new Date(actor.birthday).toLocaleDateString() }}
                                        </dd>
                                    </div>
                                    <div v-if="actor.birthplace" class="flex justify-between">
                                        <dt class="text-dim">Birthplace</dt>
                                        <dd class="text-white">{{ actor.birthplace }}</dd>
                                    </div>
                                    <div v-if="actor.ethnicity" class="flex justify-between">
                                        <dt class="text-dim">Ethnicity</dt>
                                        <dd class="text-white">{{ actor.ethnicity }}</dd>
                                    </div>
                                    <div v-if="actor.astrology" class="flex justify-between">
                                        <dt class="text-dim">Astrology</dt>
                                        <dd class="text-white">{{ actor.astrology }}</dd>
                                    </div>
                                </dl>
                            </div>

                            <!-- Career Info (mobile) -->
                            <div
                                v-if="actor.career_start_year || actor.career_end_year"
                                class="border-border bg-surface rounded-lg border p-4"
                            >
                                <h3 class="mb-3 text-xs font-semibold text-white uppercase">
                                    Career
                                </h3>
                                <dl class="space-y-2 text-sm">
                                    <div
                                        v-if="actor.career_start_year"
                                        class="flex justify-between"
                                    >
                                        <dt class="text-dim">Career Start</dt>
                                        <dd class="text-white">{{ actor.career_start_year }}</dd>
                                    </div>
                                    <div v-if="actor.career_end_year" class="flex justify-between">
                                        <dt class="text-dim">Career End</dt>
                                        <dd class="text-white">{{ actor.career_end_year }}</dd>
                                    </div>
                                </dl>
                            </div>

                            <!-- Physical Attributes (mobile) -->
                            <div
                                v-if="
                                    actor.weight_kg ||
                                    actor.cupsize ||
                                    actor.fake_boobs ||
                                    actor.same_sex_only
                                "
                                class="border-border bg-surface rounded-lg border p-4"
                            >
                                <h3 class="mb-3 text-xs font-semibold text-white uppercase">
                                    Physical
                                </h3>
                                <dl class="space-y-2 text-sm">
                                    <div v-if="actor.weight_kg" class="flex justify-between">
                                        <dt class="text-dim">Weight</dt>
                                        <dd class="text-white">{{ actor.weight_kg }} kg</dd>
                                    </div>
                                    <div v-if="actor.cupsize" class="flex justify-between">
                                        <dt class="text-dim">Cup Size</dt>
                                        <dd class="text-white">{{ actor.cupsize }}</dd>
                                    </div>
                                    <div v-if="actor.fake_boobs" class="flex justify-between">
                                        <dt class="text-dim">Enhanced</dt>
                                        <dd class="text-white">Yes</dd>
                                    </div>
                                    <div v-if="actor.same_sex_only" class="flex justify-between">
                                        <dt class="text-dim">Same-sex Only</dt>
                                        <dd class="text-white">Yes</dd>
                                    </div>
                                </dl>
                            </div>

                            <!-- Body Modifications (mobile) -->
                            <div
                                v-if="actor.tattoos || actor.piercings"
                                class="border-border bg-surface rounded-lg border p-4"
                            >
                                <h3 class="mb-3 text-xs font-semibold text-white uppercase">
                                    Modifications
                                </h3>
                                <dl class="space-y-2 text-sm">
                                    <div v-if="actor.tattoos">
                                        <dt class="text-dim mb-1">Tattoos</dt>
                                        <dd class="text-white">{{ actor.tattoos }}</dd>
                                    </div>
                                    <div v-if="actor.piercings">
                                        <dt class="text-dim mb-1">Piercings</dt>
                                        <dd class="text-white">{{ actor.piercings }}</dd>
                                    </div>
                                </dl>
                            </div>
                        </div>
                    </Transition>
                </div>

                <!-- Desktop: Detailed Info Sections (unchanged) -->
                <div class="hidden gap-6 sm:grid md:grid-cols-2">
                    <!-- Demographics -->
                    <div
                        v-if="
                            actor.birthplace || actor.ethnicity || actor.astrology || actor.birthday
                        "
                        class="border-border bg-surface rounded-lg border p-4"
                    >
                        <h3 class="mb-3 text-sm font-semibold text-white uppercase">
                            Demographics
                        </h3>
                        <dl class="space-y-2 text-sm">
                            <div v-if="actor.birthday" class="flex justify-between">
                                <dt class="text-dim">Birthday</dt>
                                <dd class="text-white">
                                    {{ new Date(actor.birthday).toLocaleDateString() }}
                                </dd>
                            </div>
                            <div v-if="actor.birthplace" class="flex justify-between">
                                <dt class="text-dim">Birthplace</dt>
                                <dd class="text-white">{{ actor.birthplace }}</dd>
                            </div>
                            <div v-if="actor.ethnicity" class="flex justify-between">
                                <dt class="text-dim">Ethnicity</dt>
                                <dd class="text-white">{{ actor.ethnicity }}</dd>
                            </div>
                            <div v-if="actor.astrology" class="flex justify-between">
                                <dt class="text-dim">Astrology</dt>
                                <dd class="text-white">{{ actor.astrology }}</dd>
                            </div>
                        </dl>
                    </div>

                    <!-- Career Info -->
                    <div
                        v-if="actor.career_start_year || actor.career_end_year"
                        class="border-border bg-surface rounded-lg border p-4"
                    >
                        <h3 class="mb-3 text-sm font-semibold text-white uppercase">Career</h3>
                        <dl class="space-y-2 text-sm">
                            <div v-if="actor.career_start_year" class="flex justify-between">
                                <dt class="text-dim">Career Start</dt>
                                <dd class="text-white">{{ actor.career_start_year }}</dd>
                            </div>
                            <div v-if="actor.career_end_year" class="flex justify-between">
                                <dt class="text-dim">Career End</dt>
                                <dd class="text-white">{{ actor.career_end_year }}</dd>
                            </div>
                        </dl>
                    </div>

                    <!-- Physical Attributes -->
                    <div
                        v-if="
                            actor.weight_kg ||
                            actor.cupsize ||
                            actor.fake_boobs ||
                            actor.same_sex_only
                        "
                        class="border-border bg-surface rounded-lg border p-4"
                    >
                        <h3 class="mb-3 text-sm font-semibold text-white uppercase">
                            Physical Attributes
                        </h3>
                        <dl class="space-y-2 text-sm">
                            <div v-if="actor.weight_kg" class="flex justify-between">
                                <dt class="text-dim">Weight</dt>
                                <dd class="text-white">{{ actor.weight_kg }} kg</dd>
                            </div>
                            <div v-if="actor.cupsize" class="flex justify-between">
                                <dt class="text-dim">Cup Size</dt>
                                <dd class="text-white">{{ actor.cupsize }}</dd>
                            </div>
                            <div v-if="actor.fake_boobs" class="flex justify-between">
                                <dt class="text-dim">Enhanced</dt>
                                <dd class="text-white">Yes</dd>
                            </div>
                            <div v-if="actor.same_sex_only" class="flex justify-between">
                                <dt class="text-dim">Same-sex Only</dt>
                                <dd class="text-white">Yes</dd>
                            </div>
                        </dl>
                    </div>

                    <!-- Body Modifications -->
                    <div
                        v-if="actor.tattoos || actor.piercings"
                        class="border-border bg-surface rounded-lg border p-4"
                    >
                        <h3 class="mb-3 text-sm font-semibold text-white uppercase">
                            Body Modifications
                        </h3>
                        <dl class="space-y-2 text-sm">
                            <div v-if="actor.tattoos">
                                <dt class="text-dim mb-1">Tattoos</dt>
                                <dd class="text-white">{{ actor.tattoos }}</dd>
                            </div>
                            <div v-if="actor.piercings">
                                <dt class="text-dim mb-1">Piercings</dt>
                                <dd class="text-white">{{ actor.piercings }}</dd>
                            </div>
                        </dl>
                    </div>
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
                    <div class="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
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
                        <select
                            v-model="scenesSort"
                            class="border-border bg-surface text-dim h-10 shrink-0 rounded-lg border
                                px-3 py-2 text-xs transition-colors focus:border-white/20
                                focus:outline-none sm:w-40"
                            @change="onSortChange"
                        >
                            <option v-for="opt in sortOptions" :key="opt.value" :value="opt.value">
                                {{ opt.label }}
                            </option>
                        </select>
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
            <ActorsEditModal
                v-if="actor && showEditModal"
                :actor="actor"
                :visible="showEditModal"
                @close="showEditModal = false"
                @updated="handleActorUpdated"
            />

            <!-- Create Modal -->
            <ActorsEditModal
                v-if="showCreateModal"
                :actor="null"
                :visible="showCreateModal"
                @close="showCreateModal = false"
                @created="handleActorCreated"
            />

            <!-- Fetch Metadata Modal -->
            <ActorsFetchMetadataModal
                v-if="showFetchModal && actor"
                :visible="showFetchModal"
                :actor-name="actor.name"
                :current-actor="actor"
                @close="showFetchModal = false"
                @apply="handleApplyMetadata"
            />

            <!-- Delete Modal -->
            <ActorsDeleteModal
                v-if="actor && showDeleteModal"
                :visible="showDeleteModal"
                :actor="actor"
                @close="showDeleteModal = false"
                @deleted="handleActorDeleted"
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
