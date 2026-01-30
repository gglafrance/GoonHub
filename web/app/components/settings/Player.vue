<script setup lang="ts">
const settingsStore = useSettingsStore();
const { message, error, clearMessages } = useSettingsMessage();

const playerAutoplay = ref(false);
const playerVolume = ref(100);
const playerLoop = ref(false);

const syncFromStore = () => {
    playerAutoplay.value = settingsStore.autoplay;
    playerVolume.value = settingsStore.defaultVolume;
    playerLoop.value = settingsStore.loop;
};

onMounted(syncFromStore);

watch(() => settingsStore.settings, syncFromStore);

const handleSavePlayer = async () => {
    clearMessages();
    try {
        await settingsStore.updatePlayer(
            playerAutoplay.value,
            playerVolume.value,
            playerLoop.value,
        );
        message.value = 'Player settings saved';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

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

                <button
                    @click="handleSavePlayer"
                    :disabled="settingsStore.isLoading"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    Save Player Settings
                </button>
            </div>
        </div>
    </div>
</template>
