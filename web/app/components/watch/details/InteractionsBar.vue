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
    <div class="flex items-center gap-2">
        <!-- Like -->
        <button
            @click="toggleLike"
            class="group flex flex-col items-center gap-1 rounded-lg border px-3 py-2
                transition-all duration-200"
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
                    liked ? 'text-lava' : 'text-white/20 group-hover:text-white/40',
                    likeAnimating ? 'scale-125' : 'scale-100',
                ]"
            >
                <Icon :name="liked ? 'heroicons:heart-solid' : 'heroicons:heart'" size="16" />
            </div>
            <span
                class="text-[10px] font-medium transition-colors duration-200"
                :class="[liked ? 'text-lava/60' : 'text-white/25 group-hover:text-white/40']"
            >
                {{ liked ? 'Liked' : 'Like' }}
            </span>
        </button>

        <!-- Jizz -->
        <button
            @click="incrementJizzed"
            class="group flex flex-col items-center gap-1 rounded-lg border px-3 py-2
                transition-all duration-200"
            :class="[
                jizzedCount > 0
                    ? 'border-white/20 bg-white/[0.05]'
                    : `border-border hover:border-border-hover bg-white/[0.02]
                        hover:bg-white/[0.04]`,
            ]"
        >
            <div
                class="transition-all duration-200"
                :class="[
                    jizzedCount > 0 ? 'text-white' : 'text-white/20 group-hover:text-white/40',
                    jizzedAnimating ? 'scale-125' : 'scale-100',
                ]"
            >
                <Icon name="fluent-emoji-high-contrast:sweat-droplets" size="16" />
            </div>
            <span
                class="text-[10px] font-medium tabular-nums transition-colors duration-200"
                :class="[
                    jizzedCount > 0 ? 'text-white/60' : 'text-white/25 group-hover:text-white/40',
                ]"
            >
                {{ jizzedCount > 0 ? jizzedCount : 'Jizz' }}
            </span>
        </button>
    </div>
</template>
