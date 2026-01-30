<script setup lang="ts">
const props = defineProps<{
    videoId: number;
    initialRating?: number;
}>();

const videoIdRef = computed(() => props.videoId);
const {
    displayRating,
    isHovering,
    getStarState,
    onStarHover,
    onStarLeave,
    onStarClick,
    setRating,
} = useVideoRating(videoIdRef);

onMounted(() => {
    if (props.initialRating) {
        setRating(props.initialRating);
    }
});

watch(
    () => props.initialRating,
    (newRating) => {
        if (newRating !== undefined) {
            setRating(newRating);
        }
    },
);
</script>

<template>
    <div class="flex flex-col items-center gap-2.5">
        <!-- Stars -->
        <div class="flex items-center gap-0.75" @mouseleave="onStarLeave">
            <div v-for="star in 5" :key="star" class="relative h-4.5 w-4.5 cursor-pointer">
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
                    :class="[isHovering ? 'text-white/30' : 'text-white/15']"
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
                        :class="[isHovering ? 'text-lava-glow' : 'text-lava']"
                    />
                </div>
            </div>
        </div>

        <!-- Rating value -->
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
