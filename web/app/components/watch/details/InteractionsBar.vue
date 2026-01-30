<script setup lang="ts">
const props = defineProps<{
    videoId: number;
    initialLiked?: boolean;
    initialJizzedCount?: number;
}>();

const videoIdRef = computed(() => props.videoId);

const { liked, animating: likeAnimating, toggle: toggleLike, setLiked } = useVideoLike(videoIdRef);
const {
    count: jizzedCount,
    animating: jizzedAnimating,
    increment: incrementJizzed,
    setCount: setJizzedCount,
} = useVideoJizzCount(videoIdRef);

onMounted(() => {
    if (props.initialLiked !== undefined) {
        setLiked(props.initialLiked);
    }
    if (props.initialJizzedCount !== undefined) {
        setJizzedCount(props.initialJizzedCount);
    }
});

watch(
    () => props.initialLiked,
    (newLiked) => {
        if (newLiked !== undefined) {
            setLiked(newLiked);
        }
    },
);

watch(
    () => props.initialJizzedCount,
    (newCount) => {
        if (newCount !== undefined) {
            setJizzedCount(newCount);
        }
    },
);
</script>

<template>
    <div class="flex w-full items-center justify-center gap-3">
        <!-- Like -->
        <button
            @click="toggleLike"
            class="group flex flex-col items-center gap-0.5 transition-all duration-200"
            title="Like this video"
        >
            <div
                class="transition-all duration-200"
                :class="[
                    liked ? 'text-lava' : 'text-white/25 group-hover:text-white/50',
                    likeAnimating ? 'scale-125' : 'scale-100',
                ]"
            >
                <Icon :name="liked ? 'heroicons:heart-solid' : 'heroicons:heart'" size="18" />
            </div>
            <span
                class="text-[9px] font-medium transition-colors duration-200"
                :class="[liked ? 'text-lava/70' : 'text-white/25 group-hover:text-white/40']"
            >
                {{ liked ? 'Liked' : 'Like' }}
            </span>
        </button>

        <!-- Jizz -->
        <button
            @click="incrementJizzed"
            class="group flex flex-col items-center gap-0.5 transition-all duration-200"
            title="Track completion"
        >
            <div
                class="transition-all duration-200"
                :class="[
                    jizzedCount > 0 ? 'text-white' : 'text-white/25 group-hover:text-white/50',
                    jizzedAnimating ? 'scale-125' : 'scale-100',
                ]"
            >
                <Icon name="fluent-emoji-high-contrast:sweat-droplets" size="18" />
            </div>
            <span
                class="text-[9px] font-medium tabular-nums transition-colors duration-200"
                :class="[
                    jizzedCount > 0 ? 'text-white/70' : 'text-white/25 group-hover:text-white/40',
                ]"
            >
                {{ jizzedCount > 0 ? jizzedCount : 'Jizz' }}
            </span>
        </button>
    </div>
</template>
