<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// quick_rate widget: recently watched scenes without rating, one card at a time: rate (half
// stars), like, or skip; rating moves on to the next card.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{
        days: number;
        total_unrated: number;
        liked: Record<string, boolean>;
    }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const { formatDuration } = useFormatter();

const index = ref(0);
const rated = ref(0);

const scenes = computed(() => props.data.scenes);
const scene = computed(() => scenes.value[index.value]);
const sceneId = computed(() => scene.value?.id);
const done = computed(() => index.value >= scenes.value.length);
const totalUnrated = computed(() => props.data.widget?.total_unrated ?? scenes.value.length);

const {
    displayRating,
    getStarState,
    onStarHover,
    onStarLeave,
    onStarClick,
    setRating,
    currentRating,
} = useSceneRating(sceneId);
const { liked, toggle: toggleLike, setLiked } = useSceneLike(sceneId);

const version = computed(() =>
    scene.value?.updated_at ? new Date(scene.value.updated_at).getTime() : '',
);

watch(
    scene,
    (s) => {
        setRating(0);
        setLiked(s ? props.data.widget?.liked?.[String(s.id)] === true : false);
    },
    { immediate: true },
);

watch(scenes, () => {
    index.value = 0;
    rated.value = 0;
});

const next = () => {
    index.value += 1;
};

const rate = async (star: number, isLeftHalf: boolean) => {
    await onStarClick(star, isLeftHalf);
    if (currentRating.value > 0) {
        rated.value += 1;
        setTimeout(next, 350);
    }
};
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-4">
        <div
            v-if="scenes.length === 0 || done"
            class="flex flex-col items-center justify-center gap-2 py-8 text-center"
        >
            <Icon name="heroicons:check-badge" size="28" class="text-emerald" />
            <p class="text-sm text-white">
                {{ scenes.length === 0 ? 'Nothing to rate' : 'All caught up' }}
            </p>
            <p class="text-dim text-xs">
                <template v-if="rated > 0">{{ rated }} rated · </template>
                Scenes watched in the last {{ data.widget?.days ?? 30 }} days
            </p>
            <button
                v-if="scenes.length > 0 && totalUnrated > scenes.length"
                class="border-border text-dim hover:border-lava/30 hover:text-lava mt-1 rounded-md
                    border px-3 py-1 text-[11px] transition-colors"
                @click="emit('refresh')"
            >
                Load more
            </button>
        </div>

        <div v-else-if="scene" class="flex flex-col gap-4 sm:flex-row">
            <NuxtLink
                :to="`/watch/${scene.id}`"
                class="bg-void relative aspect-video w-full shrink-0 overflow-hidden rounded-lg
                    sm:w-80"
            >
                <video
                    v-if="scene.preview_video_path"
                    :key="scene.id"
                    :src="`/scene-previews/${scene.id}?v=${version}`"
                    :poster="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    autoplay
                    muted
                    loop
                    playsinline
                    class="h-full w-full object-cover"
                />
                <img
                    v-else
                    :src="`/thumbnails/${scene.id}?size=lg&v=${version}`"
                    :alt="scene.title"
                    class="h-full w-full object-cover"
                />
            </NuxtLink>

            <div class="flex min-w-0 flex-1 flex-col justify-between gap-3">
                <div>
                    <div class="text-dim mb-1 font-mono text-[10px]">
                        {{ index + 1 }} / {{ totalUnrated }}
                    </div>
                    <h3 class="line-clamp-2 text-sm font-medium text-white">{{ scene.title }}</h3>
                    <div class="text-dim mt-0.5 font-mono text-[11px]">
                        {{ formatDuration(scene.duration) }}
                    </div>
                </div>

                <div class="flex items-center gap-1" @mouseleave="onStarLeave">
                    <div v-for="star in 5" :key="star" class="relative h-7 w-7 cursor-pointer">
                        <div
                            class="absolute inset-y-0 left-0 z-10 w-1/2"
                            @mouseenter="onStarHover(star, true)"
                            @click="rate(star, true)"
                        />
                        <div
                            class="absolute inset-y-0 right-0 z-10 w-1/2"
                            @mouseenter="onStarHover(star, false)"
                            @click="rate(star, false)"
                        />
                        <Icon name="heroicons:star" size="28" class="text-white/20" />
                        <Icon
                            v-if="getStarState(star) === 'full'"
                            name="heroicons:star-solid"
                            size="28"
                            class="absolute inset-0 text-amber-400"
                        />
                        <div
                            v-else-if="getStarState(star) === 'half'"
                            class="absolute inset-0 w-1/2 overflow-hidden"
                        >
                            <Icon name="heroicons:star-solid" size="28" class="text-amber-400" />
                        </div>
                    </div>
                    <span class="text-dim ml-2 font-mono text-xs">
                        {{ displayRating > 0 ? displayRating.toFixed(1) : '' }}
                    </span>
                </div>

                <div class="flex items-center gap-2">
                    <button
                        class="flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[11px]
                            transition-colors"
                        :class="
                            liked
                                ? 'border-pink-400/40 bg-pink-400/10 text-pink-400'
                                : 'border-border text-dim hover:text-pink-400'
                        "
                        @click="toggleLike"
                    >
                        <Icon
                            :name="liked ? 'heroicons:heart-solid' : 'heroicons:heart'"
                            size="13"
                        />
                        {{ liked ? 'Liked' : 'Like' }}
                    </button>
                    <button
                        class="border-border text-dim flex items-center gap-1 rounded-md border px-3
                            py-1.5 text-[11px] transition-colors hover:text-white"
                        @click="next"
                    >
                        Skip
                        <Icon name="heroicons:chevron-right" size="13" />
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
