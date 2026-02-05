<script setup lang="ts">
const settingsStore = useSettingsStore();

const playerAutoplay = computed({
    get: () => settingsStore.draft?.autoplay ?? false,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.autoplay = v;
    },
});

const playerVolume = computed({
    get: () => settingsStore.draft?.default_volume ?? 100,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.default_volume = v;
    },
});

const playerLoop = computed({
    get: () => settingsStore.draft?.loop ?? false,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.loop = v;
    },
});
</script>

<template>
    <div class="space-y-6">
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Player Preferences</h3>
            <div class="space-y-5">
                <!-- Autoplay Toggle -->
                <div class="flex items-center justify-between">
                    <div>
                        <div class="text-sm text-white">Autoplay</div>
                        <div class="text-dim text-xs">Automatically play videos when opened</div>
                    </div>
                    <UiToggle v-model="playerAutoplay" />
                </div>

                <!-- Loop Toggle -->
                <div class="flex items-center justify-between">
                    <div>
                        <div class="text-sm text-white">Loop</div>
                        <div class="text-dim text-xs">Loop videos when they finish</div>
                    </div>
                    <UiToggle v-model="playerLoop" />
                </div>

                <!-- Volume Slider -->
                <div>
                    <div class="mb-2 flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Default Volume</div>
                            <div class="text-dim text-xs">Initial volume level for videos</div>
                        </div>
                        <span class="text-dim font-mono text-xs">{{ playerVolume }}%</span>
                    </div>
                    <input
                        v-model.number="playerVolume"
                        type="range"
                        min="0"
                        max="100"
                        step="5"
                        class="accent-lava w-full"
                    />
                </div>
            </div>
        </div>
    </div>
</template>
