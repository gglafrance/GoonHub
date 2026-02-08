<script setup lang="ts">
import type { ResolvedShareLink } from '~/types/share';

definePageMeta({
    layout: false,
});

const route = useRoute();
const token = computed(() => route.params.token as string);

const { resolveShareLink } = useApiShares();
const { formatDuration } = useFormatter();

const resolved = ref<ResolvedShareLink | null>(null);
const loading = ref(true);
const errorState = ref<'not_found' | 'expired' | 'auth_required' | null>(null);

const streamUrl = computed(() => `/api/v1/shares/${token.value}/stream`);

const loadShare = async () => {
    loading.value = true;
    errorState.value = null;
    try {
        const data = await resolveShareLink(token.value);
        resolved.value = data;
    } catch (e: unknown) {
        const err = e as Error & { code?: string; status?: number };
        if (err.code === 'AUTH_REQUIRED' || err.status === 401) {
            errorState.value = 'auth_required';
        } else if (err.code === 'SHARE_LINK_EXPIRED' || err.status === 410) {
            errorState.value = 'expired';
        } else {
            errorState.value = 'not_found';
        }
    } finally {
        loading.value = false;
    }
};

onMounted(loadShare);

watch(
    () => resolved.value,
    (data) => {
        if (data) {
            useSeoMeta({
                title: data.scene.title || 'Shared Scene',
                ogTitle: data.scene.title || 'Shared Scene',
                description: data.scene.description || `Watch this shared scene on GoonHub`,
                ogImage: `/thumbnails/${data.scene.id}?size=lg`,
                ogType: 'video.other',
            });
            useHead({ title: data.scene.title || 'Shared Scene' });
        }
    },
    { immediate: true },
);
</script>

<template>
    <div class="bg-void min-h-screen">
        <!-- Loading -->
        <div v-if="loading" class="flex h-screen items-center justify-center">
            <LoadingSpinner label="Loading..." />
        </div>

        <!-- Auth Required -->
        <div
            v-else-if="errorState === 'auth_required'"
            class="flex h-screen items-center justify-center px-4"
        >
            <div class="max-w-sm text-center">
                <div
                    class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full
                        bg-amber-500/10"
                >
                    <Icon name="heroicons:lock-closed" size="28" class="text-amber-400" />
                </div>
                <h1 class="mb-2 text-lg font-semibold text-white">Login Required</h1>
                <p class="text-dim mb-6 text-sm">
                    This shared scene requires you to be logged in to view it.
                </p>
                <NuxtLink
                    :to="`/login?redirect=/share/${token}`"
                    class="bg-lava hover:bg-lava-glow inline-block rounded-lg px-6 py-2.5 text-sm
                        font-semibold text-white transition-all"
                >
                    Log In
                </NuxtLink>
            </div>
        </div>

        <!-- Expired or Not Found -->
        <div
            v-else-if="errorState === 'expired' || errorState === 'not_found'"
            class="flex h-screen items-center justify-center px-4"
        >
            <div class="max-w-sm text-center">
                <div
                    class="bg-lava/10 mx-auto mb-4 flex h-16 w-16 items-center justify-center
                        rounded-full"
                >
                    <Icon name="heroicons:link-slash" size="28" class="text-lava" />
                </div>
                <h1 class="mb-2 text-lg font-semibold text-white">Link Unavailable</h1>
                <p class="text-dim mb-6 text-sm">
                    {{
                        errorState === 'expired'
                            ? 'This share link has expired and is no longer available.'
                            : 'This share link does not exist or has been removed.'
                    }}
                </p>
                <NuxtLink
                    to="/"
                    class="border-border text-dim hover:border-border-hover inline-block rounded-lg
                        border px-6 py-2.5 text-sm font-medium transition-all hover:text-white"
                >
                    Go Home
                </NuxtLink>
            </div>
        </div>

        <!-- Resolved: Show Player + Metadata -->
        <div v-else-if="resolved" class="mx-auto max-w-6xl px-4 py-8">
            <!-- Player -->
            <div class="mb-6">
                <div class="bg-surface/30 overflow-hidden rounded-xl">
                    <LazyScenePlayer
                        :scene-url="streamUrl"
                        :poster-url="`/thumbnails/${resolved.scene.id}?size=lg`"
                        :autoplay="false"
                    />
                </div>
            </div>

            <!-- Metadata -->
            <div class="space-y-4">
                <!-- Title -->
                <h1 class="text-lg font-semibold text-white">
                    {{ resolved.scene.title }}
                </h1>

                <!-- Info Row -->
                <div class="text-dim flex flex-wrap items-center gap-4 text-xs">
                    <span v-if="resolved.scene.duration" class="flex items-center gap-1.5">
                        <Icon name="heroicons:clock" size="12" />
                        {{ formatDuration(resolved.scene.duration) }}
                    </span>
                    <span v-if="resolved.scene.studio" class="flex items-center gap-1.5">
                        <Icon name="heroicons:building-office" size="12" />
                        {{ resolved.scene.studio }}
                    </span>
                    <span v-if="resolved.scene.release_date" class="flex items-center gap-1.5">
                        <Icon name="heroicons:calendar" size="12" />
                        <NuxtTime
                            :datetime="resolved.scene.release_date"
                            year="numeric"
                            month="short"
                            day="numeric"
                        />
                    </span>
                </div>

                <!-- Description -->
                <p
                    v-if="resolved.scene.description"
                    class="text-dim/70 max-w-3xl text-sm leading-relaxed"
                >
                    {{ resolved.scene.description }}
                </p>

                <!-- Tags -->
                <div
                    v-if="resolved.scene.tags && resolved.scene.tags.length > 0"
                    class="flex flex-wrap gap-1.5"
                >
                    <span
                        v-for="tag in resolved.scene.tags"
                        :key="tag"
                        class="border-border/50 bg-surface/40 text-dim rounded-md border px-2 py-1
                            text-[11px]"
                    >
                        {{ tag }}
                    </span>
                </div>

                <!-- Actors -->
                <div
                    v-if="resolved.scene.actors && resolved.scene.actors.length > 0"
                    class="flex flex-wrap items-center gap-2"
                >
                    <span class="text-dim text-[10px] font-medium tracking-wider uppercase">
                        Actors
                    </span>
                    <span
                        v-for="actor in resolved.scene.actors"
                        :key="actor"
                        class="text-dim/80 text-xs"
                    >
                        {{ actor }}
                    </span>
                </div>
            </div>

            <!-- Branding -->
            <div class="border-border/30 text-dim/30 mt-12 border-t pt-4 text-center text-[10px]">
                Shared via GoonHub
            </div>
        </div>
    </div>
</template>
