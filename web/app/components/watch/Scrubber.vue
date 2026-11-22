<script setup lang="ts">
import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';
import type { ScrubberMode } from '~/types/settings';
import type FreeStrip from '~/components/watch/scrubber/FreeStrip.vue';

type Player = ReturnType<typeof videojs>;

// Thumbnail scrubber strip under the player (like Stash's scene scrubber): the sprite sheet
// thumbnails side by side. Two modes (Settings > Player, or the switch in the header): free (the
// strip scrolls on its own, a click seeks) and centered (the centre line is the playback
// position, moving the strip seeks the video). Only the visible thumbnails are rendered.
const props = defineProps<{
    player: Player | null;
    cues: VttCue[];
    markers: Marker[];
    // whether the scene has sprite sheets (false: not generated yet)
    hasSprites: boolean;
    // the player plays the file itself (false: converted or remuxed on the fly)
    directStream?: boolean;
}>();

const emit = defineEmits<{
    seek: [time: number];
    playMarker: [marker: Marker];
    hide: [];
}>();

const settingsStore = useSettingsStore();

// The cues are only read: skip the reactive proxies of thousands of objects
const cueList = computed<VttCue[]>(() => toRaw(props.cues));
const count = computed(() => cueList.value.length);
const tileHeight = SCRUBBER_TILE_HEIGHT;
const tileWidth = computed(() => {
    const first = cueList.value[0];
    return first ? scrubberTileWidth(first.w, first.h, tileHeight) : 0;
});
const markerLaneHeight = computed(() => (props.markers.length ? 18 : 0));

const mode = computed(() => settingsStore.scrubberMode);
const modes: { value: ScrubberMode; label: string; icon: string; title: string }[] = [
    {
        value: 'free',
        label: 'Free',
        icon: 'heroicons:arrows-right-left',
        title: 'Free: scroll the strip on its own, click a thumbnail to jump to it',
    },
    {
        value: 'centered',
        label: 'Centered',
        icon: 'heroicons:viewfinder-circle',
        title: 'Centered (like Stash): the middle is the current time, drag or scroll to seek',
    },
];
const setMode = (value: ScrubberMode) => {
    if (value !== mode.value) settingsStore.setScrubberMode(value);
};

const freeStrip = ref<InstanceType<typeof FreeStrip> | null>(null);
const followPaused = computed(() => mode.value === 'free' && !!freeStrip.value?.followPaused);
</script>

<template>
    <div class="scrubber border-border bg-surface/60 rounded-xl border">
        <div class="flex h-7 items-center gap-2 px-2.5">
            <Icon name="heroicons:film" size="13" class="text-lava" />
            <span class="text-[11px] font-medium text-white">Scrubber</span>
            <span v-if="count" class="text-dim font-mono text-[10px]">
                {{ count }} thumbnails
            </span>
            <span v-else-if="!hasSprites" class="text-dim text-[11px]">
                Thumbnails not generated yet
            </span>
            <span v-else class="text-dim text-[11px]">Loading thumbnails...</span>
            <div class="ml-auto flex items-center gap-1">
                <button
                    v-if="count && followPaused"
                    type="button"
                    class="border-lava/30 bg-lava/10 text-lava hover:bg-lava/20 flex h-5
                        items-center gap-1 rounded-md border px-1.5 text-[10px] font-medium
                        transition-colors"
                    title="Follow playback"
                    @click="freeStrip?.resumeFollow()"
                >
                    <Icon name="heroicons:arrows-pointing-in" size="11" />
                    Follow
                </button>
                <div
                    class="border-border flex h-5 items-center rounded-md border p-px"
                    role="group"
                    aria-label="Scrubber mode"
                >
                    <button
                        v-for="opt in modes"
                        :key="opt.value"
                        type="button"
                        class="flex h-full items-center gap-1 rounded-[5px] px-1.5 text-[10px]
                            font-medium transition-colors"
                        :class="
                            mode === opt.value
                                ? 'bg-lava/15 text-lava'
                                : 'text-dim hover:bg-white/5 hover:text-white'
                        "
                        :title="opt.title"
                        :aria-pressed="mode === opt.value"
                        @click="setMode(opt.value)"
                    >
                        <Icon :name="opt.icon" size="11" />
                        <span class="hidden sm:inline">{{ opt.label }}</span>
                    </button>
                </div>
                <button
                    type="button"
                    class="text-dim flex h-5 w-5 items-center justify-center rounded-md
                        transition-colors hover:bg-white/5 hover:text-white"
                    title="Hide the scrubber"
                    aria-label="Hide the scrubber"
                    @click="emit('hide')"
                >
                    <Icon name="heroicons:x-mark" size="13" />
                </button>
            </div>
        </div>

        <template v-if="count">
            <WatchScrubberCenteredStrip
                v-if="mode === 'centered'"
                :player="player"
                :cues="cueList"
                :markers="markers"
                :tile-width="tileWidth"
                :tile-height="tileHeight"
                :marker-lane-height="markerLaneHeight"
                :direct-stream="directStream ?? true"
                @seek="emit('seek', $event)"
                @play-marker="emit('playMarker', $event)"
            />
            <WatchScrubberFreeStrip
                v-else
                ref="freeStrip"
                :player="player"
                :cues="cueList"
                :markers="markers"
                :tile-width="tileWidth"
                :tile-height="tileHeight"
                :marker-lane-height="markerLaneHeight"
                @seek="emit('seek', $event)"
                @play-marker="emit('playMarker', $event)"
            />
        </template>
    </div>
</template>
