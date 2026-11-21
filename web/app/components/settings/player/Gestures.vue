<script setup lang="ts">
const settingsStore = useSettingsStore();

const gesturesEnabled = computed({
    get: () => settingsStore.draft?.player_gestures ?? true,
    set: (v: boolean) => {
        if (settingsStore.draft) settingsStore.draft.player_gestures = v;
    },
});

const seekStep = computed({
    get: () => normalizeGestureSeekStep(settingsStore.draft?.gesture_seek_step),
    set: (v: number) => {
        if (settingsStore.draft) settingsStore.draft.gesture_seek_step = v;
    },
});
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-5 text-sm font-semibold text-white">Touch Gestures</h3>
        <div class="space-y-5">
            <div class="flex items-center justify-between gap-4">
                <div>
                    <div class="text-sm text-white">Player Gestures</div>
                    <div class="text-dim text-xs">
                        On touch screens: double-tap the sides to seek, swipe sideways to scrub,
                        hold for 2x speed; in fullscreen, swipe up or down on the right for volume
                        and on the left for brightness
                    </div>
                </div>
                <UiToggle v-model="gesturesEnabled" />
            </div>

            <div v-if="gesturesEnabled">
                <div class="mb-2">
                    <div class="text-sm text-white">Double-Tap Seek</div>
                    <div class="text-dim text-xs">
                        Seconds skipped by each tap; repeated taps add up
                    </div>
                </div>
                <div class="flex gap-2">
                    <button
                        v-for="step in GESTURE_SEEK_STEPS"
                        :key="step"
                        type="button"
                        class="rounded-lg border px-3 py-1.5 font-mono text-xs font-medium
                            transition-all"
                        :class="
                            seekStep === step
                                ? 'border-lava/40 bg-lava/10 text-lava'
                                : 'border-border bg-void/50 text-dim hover:text-white'
                        "
                        @click="seekStep = step"
                    >
                        {{ step }}s
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
