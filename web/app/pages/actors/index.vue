<script setup lang="ts">
import type { Actor, ActorListItem } from '~/types/actor';

useHead({ title: 'Actors' });

useSeoMeta({
    title: 'Actors',
    ogTitle: 'Actors - GoonHub',
    description: 'Browse actors in your video library',
    ogDescription: 'Browse actors in your video library',
});

const api = useApi();
const router = useRouter();
const authStore = useAuthStore();

const actors = ref<ActorListItem[]>([]);
const total = ref(0);
const currentPage = ref(1);
const limit = ref(20);
const searchQuery = ref('');
const isLoading = ref(false);
const error = ref<string | null>(null);
const showCreateModal = ref(false);

const isAdmin = computed(() => authStore.user?.role === 'admin');

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

const loadActors = async (page = 1) => {
    isLoading.value = true;
    error.value = null;
    try {
        const response = await api.fetchActors(page, limit.value, searchQuery.value);
        actors.value = response.data;
        total.value = response.total;
        currentPage.value = page;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load actors';
    } finally {
        isLoading.value = false;
    }
};

onMounted(() => {
    loadActors();
});

watch(
    () => currentPage.value,
    (newPage) => {
        loadActors(newPage);
    },
);

watch(searchQuery, () => {
    if (searchTimeout) {
        clearTimeout(searchTimeout);
    }
    searchTimeout = setTimeout(() => {
        loadActors(1);
    }, 300);
});

const handleActorCreated = (newActor: Actor) => {
    showCreateModal.value = false;
    router.push(`/actors/${newActor.uuid}`);
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Header -->
            <div class="mb-6">
                <div class="flex items-center justify-between">
                    <h1 class="text-lg font-semibold text-white">Actors</h1>
                    <div class="flex items-center gap-2">
                        <button
                            v-if="isAdmin"
                            @click="showCreateModal = true"
                            class="bg-lava hover:bg-lava-glow flex items-center gap-1 rounded-lg
                                px-3 py-1.5 text-xs font-semibold text-white transition-colors"
                        >
                            <Icon name="heroicons:plus" size="14" />
                            New Actor
                        </button>
                        <span
                            class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                                font-mono text-[11px]"
                        >
                            {{ total }} actors
                        </span>
                    </div>
                </div>

                <!-- Search bar -->
                <div class="mt-4">
                    <div class="relative">
                        <Icon
                            name="heroicons:magnifying-glass"
                            size="16"
                            class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                        />
                        <input
                            v-model="searchQuery"
                            type="text"
                            placeholder="Search actors..."
                            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20
                                w-full rounded-lg border py-2 pr-3 pl-9 text-sm text-white
                                placeholder-white/40 transition-all focus:ring-2 focus:outline-none"
                        />
                    </div>
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="error" :message="error" class="mb-4" />

            <!-- Loading State -->
            <div
                v-if="isLoading && actors.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading actors..." />
            </div>

            <!-- Empty State -->
            <div
                v-else-if="actors.length === 0"
                class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:user-group" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No actors found</p>
                <p v-if="searchQuery" class="text-dim mt-1 text-xs">Try a different search term</p>
            </div>

            <!-- Actor Grid -->
            <div v-else>
                <ActorGrid :actors="actors" />

                <Pagination v-model="currentPage" :total="total" :limit="limit" />
            </div>

            <!-- Create Modal -->
            <ActorsEditModal
                v-if="showCreateModal"
                :actor="null"
                :visible="showCreateModal"
                @close="showCreateModal = false"
                @created="handleActorCreated"
            />
        </div>
    </div>
</template>
