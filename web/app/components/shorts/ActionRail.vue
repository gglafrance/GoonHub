<script setup lang="ts">
// Like, rating, jizz, video info, sound, open the full scene, and maximize,
// stacked on the right of a short.
const props = defineProps<{
    sceneId: number;
    liked: boolean;
    rating: number;
    jizzCount: number;
    muted: boolean;
    maximized: boolean;
    infoOpen: boolean;
}>();

const emit = defineEmits<{
    toggleMute: [];
    toggleMaximize: [];
    toggleInfo: [];
}>();

const id = computed(() => props.sceneId);
const like = useSceneLike(id);
const stars = useSceneRating(id);
const jizz = useSceneJizzCount(id);

const showRating = ref(false);

// Seed from the feed's side maps; the composables own the state after that
watch(
    () => [props.sceneId, props.liked, props.rating, props.jizzCount] as const,
    ([, liked, r, count]) => {
        like.setLiked(liked);
        stars.setRating(r);
        jizz.setCount(count);
    },
    { immediate: true },
);

const onStarMove = (star: number, e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    stars.onStarHover(star, e.clientX - rect.left < rect.width / 2);
};

const onStarClick = async (star: number, e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    await stars.onStarClick(star, e.clientX - rect.left < rect.width / 2);
    showRating.value = false;
};
</script>

<template>
    <div class="flex flex-col items-center gap-3">
        <button class="rail-btn" :title="like.liked.value ? 'Unlike' : 'Like'" @click="like.toggle">
            <Icon
                :name="like.liked.value ? 'heroicons:heart-solid' : 'heroicons:heart'"
                size="22"
                :class="[
                    like.liked.value ? 'text-lava' : 'text-white',
                    like.animating.value ? 'scale-125' : '',
                ]"
                class="transition-transform"
            />
        </button>

        <div class="relative">
            <button class="rail-btn flex-col" title="Rate" @click="showRating = !showRating">
                <Icon
                    :name="
                        stars.currentRating.value > 0 ? 'heroicons:star-solid' : 'heroicons:star'
                    "
                    size="20"
                    :class="stars.currentRating.value > 0 ? 'text-amber-400' : 'text-white'"
                />
                <span v-if="stars.currentRating.value > 0" class="rail-count">
                    {{ stars.currentRating.value }}
                </span>
            </button>
            <div
                v-if="showRating"
                class="border-border absolute top-1/2 right-12 flex -translate-y-1/2 gap-0.5
                    rounded-full border bg-black/80 px-2 py-1.5 backdrop-blur-md"
                @mouseleave="stars.onStarLeave"
            >
                <button
                    v-for="star in 5"
                    :key="star"
                    class="flex h-6 w-6 items-center justify-center"
                    @mousemove="onStarMove(star, $event)"
                    @click="onStarClick(star, $event)"
                >
                    <Icon
                        :name="
                            stars.getStarState(star) === 'full'
                                ? 'heroicons:star-solid'
                                : stars.getStarState(star) === 'half'
                                  ? 'heroicons:star-solid'
                                  : 'heroicons:star'
                        "
                        size="18"
                        :class="
                            stars.getStarState(star) === 'empty'
                                ? 'text-white/50'
                                : stars.getStarState(star) === 'half'
                                  ? 'text-amber-400/60'
                                  : 'text-amber-400'
                        "
                    />
                </button>
            </div>
        </div>

        <button class="rail-btn flex-col" title="Jizzed" @click="jizz.increment">
            <Icon
                name="fluent-emoji-high-contrast:sweat-droplets"
                size="20"
                :class="[
                    jizz.count.value > 0 ? 'text-lava' : 'text-white',
                    jizz.animating.value ? 'scale-125' : '',
                ]"
                class="transition-transform"
            />
            <span v-if="jizz.count.value > 0" class="rail-count">{{ jizz.count.value }}</span>
        </button>

        <button
            class="rail-btn"
            :class="{ 'rail-btn-on': infoOpen }"
            title="Video info (I)"
            @click="emit('toggleInfo')"
        >
            <Icon
                :name="
                    infoOpen ? 'heroicons:information-circle-solid' : 'heroicons:information-circle'
                "
                size="22"
                :class="infoOpen ? 'text-lava' : 'text-white'"
            />
        </button>

        <button
            class="rail-btn"
            :title="muted ? 'Unmute (M)' : 'Mute (M)'"
            @click="emit('toggleMute')"
        >
            <Icon
                :name="muted ? 'heroicons:speaker-x-mark' : 'heroicons:speaker-wave'"
                size="20"
                class="text-white"
            />
        </button>

        <NuxtLink :to="`/watch/${sceneId}`" class="rail-btn" title="Open full scene">
            <Icon name="heroicons:arrow-top-right-on-square" size="20" class="text-white" />
        </NuxtLink>

        <button
            class="rail-btn"
            :title="maximized ? 'Leave maximize (Esc)' : 'Maximize (F)'"
            @click="emit('toggleMaximize')"
        >
            <Icon
                :name="maximized ? 'heroicons:arrows-pointing-in' : 'heroicons:arrows-pointing-out'"
                size="20"
                class="text-white"
            />
        </button>
    </div>
</template>

<style scoped>
.rail-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    min-height: 44px;
    border-radius: 9999px;
    background: rgba(0, 0, 0, 0.45);
    border: 1px solid rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(10px);
    transition:
        border-color 0.15s ease,
        background 0.15s ease;
}

.rail-btn:hover {
    border-color: rgba(255, 77, 77, 0.45);
    background: rgba(255, 77, 77, 0.12);
}

.rail-btn-on {
    border-color: rgba(255, 77, 77, 0.45);
}

.rail-count {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 9px;
    line-height: 1;
    color: rgba(255, 255, 255, 0.85);
    margin-top: 2px;
}
</style>
