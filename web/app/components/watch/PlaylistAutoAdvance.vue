<script setup lang="ts">
import type { PlaylistSceneEntry } from '~/types/playlist';

defineProps<{
    visible: boolean;
    nextScene: PlaylistSceneEntry | null;
    countdownRemaining: number;
}>();

const emit = defineEmits<{
    playNow: [];
    cancel: [];
}>();

const { formatDuration } = useFormatter();
</script>

<template>
    <Transition
        enter-active-class="transition duration-300 ease-out"
        enter-from-class="opacity-0 translate-y-4"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition duration-200 ease-in"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 translate-y-4"
    >
        <div
            v-if="visible && nextScene"
            class="glass-panel border-border fixed right-4 bottom-4 z-50 w-80 border p-4"
        >
            <div class="mb-3 flex items-center justify-between">
                <span class="text-xs font-semibold text-white">Up Next</span>
                <span class="text-lava font-mono text-xs font-bold">
                    {{ countdownRemaining }}s
                </span>
            </div>

            <div class="mb-3 flex items-center gap-3">
                <!-- Thumbnail -->
                <div class="bg-void relative h-12 w-20 shrink-0 overflow-hidden rounded">
                    <img
                        v-if="nextScene.scene.thumbnail_path"
                        :src="`/thumbnails/${nextScene.scene.id}`"
                        class="h-full w-full object-cover"
                        :alt="nextScene.scene.title"
                    />
                </div>
                <div class="min-w-0 flex-1">
                    <div class="truncate text-xs font-medium text-white">
                        {{ nextScene.scene.title }}
                    </div>
                    <div class="text-dim font-mono text-[10px]">
                        {{ formatDuration(nextScene.scene.duration) }}
                    </div>
                </div>
            </div>

            <!-- Countdown progress bar -->
            <div class="mb-3 h-1 w-full overflow-hidden rounded-full bg-white/10">
                <div
                    class="bg-lava h-full rounded-full transition-all duration-1000 ease-linear"
                    :style="{ width: `${(countdownRemaining / 5) * 100}%` }"
                ></div>
            </div>

            <div class="flex gap-2">
                <button
                    class="bg-lava hover:bg-lava-glow flex-1 rounded-lg py-1.5 text-xs font-semibold
                        text-white transition-all"
                    @click="emit('playNow')"
                >
                    Play Now
                </button>
                <button
                    class="border-border bg-surface text-dim flex-1 rounded-lg border py-1.5 text-xs
                        font-medium transition-all hover:text-white"
                    @click="emit('cancel')"
                >
                    Cancel
                </button>
            </div>
        </div>
    </Transition>
</template>
