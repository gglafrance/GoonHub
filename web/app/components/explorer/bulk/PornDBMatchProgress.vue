<script setup lang="ts">
import type { ApplyPhase } from '~/types/bulk-match';

const props = defineProps<{
    phase: ApplyPhase;
    progress: { current: number; total: number; failed: number };
}>();

const emit = defineEmits<{
    close: [];
}>();

const progressPercent = computed(() => {
    if (props.progress.total === 0) return 0;
    return Math.round((props.progress.current / props.progress.total) * 100);
});

const successCount = computed(() => props.progress.current - props.progress.failed);
</script>

<template>
    <Transition name="overlay">
        <div
            v-if="phase !== 'idle'"
            class="absolute inset-0 z-20 flex flex-col items-center justify-center overflow-hidden
                rounded-xl"
        >
            <!-- Blurred darkened background -->
            <div class="absolute inset-0 bg-black/80 backdrop-blur-md" />

            <!-- Animated radial glow that pulses while applying -->
            <div class="absolute inset-0" :class="phase === 'applying' ? 'animate-pulse-slow' : ''">
                <div
                    class="absolute top-1/2 left-1/2 h-[500px] w-[500px] -translate-x-1/2
                        -translate-y-1/2 rounded-full opacity-20 blur-3xl transition-colors
                        duration-1000"
                    :class="phase === 'done' ? 'bg-emerald-500' : 'bg-lava'"
                />
            </div>

            <!-- Scanning line that sweeps down during apply -->
            <div
                v-if="phase === 'applying'"
                class="animate-scan absolute right-0 left-0 h-px"
                style="
                    background: linear-gradient(
                        90deg,
                        transparent,
                        rgba(255, 77, 77, 0.4),
                        transparent
                    );
                "
            />

            <!-- Content -->
            <div class="relative z-10 flex flex-col items-center gap-8 px-8">
                <!-- Large percentage display -->
                <div class="relative flex flex-col items-center">
                    <span
                        class="text-[72px] leading-none font-extralight tracking-tighter
                            tabular-nums transition-colors duration-700"
                        :class="phase === 'done' ? 'text-emerald-400' : 'text-white'"
                    >
                        {{ progressPercent }}
                    </span>
                    <span
                        class="mt-1 text-lg font-extralight tracking-widest transition-colors
                            duration-700"
                        :class="phase === 'done' ? 'text-emerald-400/60' : 'text-white/30'"
                    >
                        %
                    </span>
                </div>

                <!-- Progress bar -->
                <div class="w-72">
                    <div class="h-[3px] overflow-hidden rounded-full bg-white/10">
                        <div
                            class="h-full rounded-full transition-all duration-500 ease-out"
                            :class="phase === 'done' ? 'bg-emerald-400' : 'bg-lava'"
                            :style="{ width: `${progressPercent}%` }"
                        />
                    </div>
                </div>

                <!-- Status text -->
                <div class="flex flex-col items-center gap-2">
                    <p
                        v-if="phase === 'applying'"
                        class="text-dim text-sm font-light tracking-wide"
                    >
                        Applying metadata...
                    </p>
                    <p
                        v-else-if="phase === 'done'"
                        class="text-sm font-light tracking-wide text-emerald-400"
                    >
                        Complete
                    </p>

                    <!-- Counter pills -->
                    <div class="mt-1 flex items-center gap-3">
                        <span class="text-xs text-white/50 tabular-nums">
                            {{ progress.current }}
                            <span class="text-white/20">/</span>
                            {{ progress.total }}
                            scenes
                        </span>
                        <span
                            v-if="progress.failed > 0"
                            class="inline-flex items-center gap-1 rounded-full bg-red-500/15 px-2
                                py-0.5 text-xs text-red-400"
                        >
                            {{ progress.failed }} failed
                        </span>
                    </div>
                </div>

                <!-- Done summary + close -->
                <div v-if="phase === 'done'" class="mt-2 flex flex-col items-center gap-4">
                    <div class="flex items-center gap-4 text-xs">
                        <span
                            class="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/15
                                px-3 py-1 font-medium text-emerald-400"
                        >
                            <Icon name="heroicons:check-circle-16-solid" size="13" />
                            {{ successCount }} applied
                        </span>
                        <span
                            v-if="progress.failed > 0"
                            class="inline-flex items-center gap-1.5 rounded-full bg-red-500/15 px-3
                                py-1 font-medium text-red-400"
                        >
                            <Icon name="heroicons:exclamation-triangle-16-solid" size="13" />
                            {{ progress.failed }} failed
                        </span>
                    </div>
                    <button
                        class="border-border hover:border-lava/40 hover:text-lava text-dim mt-1
                            rounded-lg border px-5 py-2 text-xs font-medium transition-all"
                        @click="emit('close')"
                    >
                        Continue
                    </button>
                </div>
            </div>
        </div>
    </Transition>
</template>

<style scoped>
.overlay-enter-active {
    transition: opacity 0.3s ease;
}
.overlay-leave-active {
    transition: opacity 0.25s ease;
}
.overlay-enter-from,
.overlay-leave-to {
    opacity: 0;
}

@keyframes scan {
    0% {
        top: 0;
        opacity: 0;
    }
    10% {
        opacity: 1;
    }
    90% {
        opacity: 1;
    }
    100% {
        top: 100%;
        opacity: 0;
    }
}
.animate-scan {
    animation: scan 3s ease-in-out infinite;
}

@keyframes pulse-slow {
    0%,
    100% {
        opacity: 1;
    }
    50% {
        opacity: 0.6;
    }
}
.animate-pulse-slow {
    animation: pulse-slow 3s ease-in-out infinite;
}
</style>
