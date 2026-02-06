<script setup lang="ts">
import type { PlaylistAutoAdvance } from '~/types/settings';

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

const playlistAutoAdvance = computed({
    get: () => settingsStore.draft?.playlist_auto_advance ?? 'countdown',
    set: (v: PlaylistAutoAdvance) => {
        if (settingsStore.draft) settingsStore.draft.playlist_auto_advance = v;
    },
});

const playlistCountdownSeconds = computed({
    get: () => settingsStore.draft?.playlist_countdown_seconds ?? 5,
    set: (v: number) => {
        if (settingsStore.draft)
            settingsStore.draft.playlist_countdown_seconds = Math.min(15, Math.max(3, v));
    },
});

const autoAdvanceOptions: { value: PlaylistAutoAdvance; label: string; desc: string }[] = [
    { value: 'instant', label: 'Instant', desc: 'Play next scene immediately' },
    { value: 'countdown', label: 'Countdown', desc: 'Show countdown before advancing' },
    { value: 'manual', label: 'Manual', desc: 'Wait for user action' },
];
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

        <!-- Playlist Player Settings -->
        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">Playlist Auto-Advance</h3>
            <div class="space-y-5">
                <!-- Auto-advance mode -->
                <div>
                    <div class="mb-2">
                        <div class="text-sm text-white">Advance Mode</div>
                        <div class="text-dim text-xs">
                            What happens when a scene ends during playlist playback
                        </div>
                    </div>
                    <div class="flex gap-2">
                        <button
                            v-for="opt in autoAdvanceOptions"
                            :key="opt.value"
                            class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-all"
                            :class="
                                playlistAutoAdvance === opt.value
                                    ? 'border-lava/40 bg-lava/10 text-lava'
                                    : 'border-border bg-void/50 text-dim hover:text-white'
                            "
                            :title="opt.desc"
                            @click="playlistAutoAdvance = opt.value"
                        >
                            {{ opt.label }}
                        </button>
                    </div>
                </div>

                <!-- Countdown duration -->
                <div v-if="playlistAutoAdvance === 'countdown'">
                    <div class="mb-2 flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Countdown Duration</div>
                            <div class="text-dim text-xs">
                                Seconds before auto-advancing to next scene
                            </div>
                        </div>
                        <span class="text-dim font-mono text-xs"
                            >{{ playlistCountdownSeconds }}s</span
                        >
                    </div>
                    <input
                        v-model.number="playlistCountdownSeconds"
                        type="range"
                        min="3"
                        max="15"
                        step="1"
                        class="accent-lava w-full"
                    />
                </div>
            </div>
        </div>
    </div>
</template>
