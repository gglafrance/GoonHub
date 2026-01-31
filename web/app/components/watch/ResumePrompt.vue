<script setup lang="ts">
const props = defineProps<{
    visible: boolean;
    resumePosition: number;
    isPlaying: boolean;
}>();

const emit = defineEmits<{
    resume: [];
    startOver: [];
    dismiss: [];
}>();

const { formatDuration } = useFormatter();

// Auto-hide 5 seconds after video starts playing
let hideTimer: ReturnType<typeof setTimeout> | null = null;

watch(
    () => props.isPlaying,
    (playing) => {
        if (playing && props.visible) {
            hideTimer = setTimeout(() => {
                emit('dismiss');
            }, 5000);
        }
    },
);

onUnmounted(() => {
    if (hideTimer) {
        clearTimeout(hideTimer);
    }
});
</script>

<template>
    <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
    >
        <div v-if="visible" class="absolute inset-x-0 top-0 z-20 p-3">
            <div class="border-lava/30 bg-void/75 rounded-lg border px-4 py-3 backdrop-blur-md">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <Icon name="heroicons:play-circle" size="20" class="text-lava" />
                        <div>
                            <span class="text-xs font-medium text-white"> Resume watching? </span>
                            <span class="text-dim ml-2 text-[11px]">
                                You left off at
                                {{ formatDuration(resumePosition) }}
                            </span>
                        </div>
                    </div>
                    <div class="flex items-center gap-2">
                        <button
                            class="text-dim px-3 py-1.5 text-[11px] font-medium transition-colors
                                hover:text-white"
                            @click="emit('startOver')"
                        >
                            Start Over
                        </button>
                        <button
                            class="bg-lava hover:bg-lava/80 rounded-md px-3 py-1.5 text-[11px]
                                font-medium text-white transition-colors"
                            @click="emit('resume')"
                        >
                            Resume
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </Transition>
</template>
