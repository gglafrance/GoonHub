<script setup lang="ts">
import type ScenePlayer from '~/components/ScenePlayer.vue';
import type { Scene } from '~/types/scene';

// Room player: the usual ScenePlayer (stream, on-the-fly conversion, shortcuts) in controlled
// mode, remounted for each queue item and kept in sync by useTogetherSync.
const room = useInjectedTogetherRoom();
const { socket, currentItem, canControl } = room;
const { fetchScene } = useApiScenes();
const settingsStore = useSettingsStore();

const scene = ref<Scene | null>(null);
const sceneError = ref('');
const loadingScene = ref(false);
const mountedItemId = ref<number | null>(null);
const startTime = ref(0);
const playerRef = ref<InstanceType<typeof ScenePlayer> | null>(null);

let loadToken = 0;
watch(
    () => (currentItem.value ? `${currentItem.value.id}:${currentItem.value.scene_id}` : ''),
    async () => {
        const item = currentItem.value;
        const token = ++loadToken;
        if (!item) {
            scene.value = null;
            mountedItemId.value = null;
            sceneError.value = '';
            return;
        }
        if (item.id === mountedItemId.value) return;
        loadingScene.value = true;
        sceneError.value = '';
        try {
            const s =
                scene.value?.id === item.scene_id
                    ? scene.value
                    : ((await fetchScene(item.scene_id)) as Scene);
            if (token !== loadToken) return;
            const st = socket.state.value;
            // The player starts where the lobby is (late joiners, item changes)
            startTime.value = st
                ? expectedPosition(st, socket.serverNow(), itemEnd(item))
                : (item.start ?? 0);
            scene.value = s;
            mountedItemId.value = item.id;
        } catch (e: unknown) {
            if (token !== loadToken) return;
            scene.value = null;
            mountedItemId.value = null;
            sceneError.value = e instanceof Error ? e.message : 'Failed to load the scene';
        } finally {
            if (token === loadToken) loadingScene.value = false;
        }
    },
    { immediate: true },
);

const player = computed(() => playerRef.value?.player ?? null);
const { autoplayBlocked, unmute, reportEnded } = useTogetherSync({
    player,
    transcoding: computed(() => !!playerRef.value?.transcodeActive),
    room,
    itemId: mountedItemId,
});

useScenePlayerShortcuts({
    player,
    scene,
    onTheaterModeToggle: () => settingsStore.toggleTheaterMode(),
    onABLoopToggle: () => playerRef.value?.abLoop?.toggle(),
});

const aspectRatio = computed(() =>
    scene.value?.width && scene.value?.height
        ? `${scene.value.width} / ${scene.value.height}`
        : '16 / 9',
);
const streamUrl = computed(() => (scene.value ? `/api/v1/scenes/${scene.value.id}/stream` : ''));
const transcodeUrl = computed(() =>
    scene.value ? `/api/v1/scenes/${scene.value.id}/transcode` : '',
);
const posterUrl = computed(() => {
    if (!scene.value?.thumbnail_path) return '';
    const v = scene.value.updated_at ? new Date(scene.value.updated_at).getTime() : 0;
    return `/thumbnails/${scene.value.id}?size=lg${v ? `&v=${v}` : ''}`;
});

const onHold = computed(() => !!socket.state.value?.hold);
const skip = () => socket.send({ t: 'skip' });
</script>

<template>
    <div
        class="border-border bg-void relative overflow-hidden rounded-xl border"
        :style="{ aspectRatio }"
    >
        <div
            v-if="!currentItem"
            class="flex h-full flex-col items-center justify-center gap-2 text-center"
        >
            <Icon name="heroicons:film" size="28" class="text-dim" />
            <p class="text-sm font-medium text-white">The queue is empty</p>
            <p class="text-dim text-xs">Add scenes to the queue to start watching together.</p>
        </div>
        <div v-else-if="loadingScene" class="flex h-full items-center justify-center">
            <LoadingSpinner label="Loading scene..." />
        </div>
        <div
            v-else-if="sceneError"
            class="flex h-full flex-col items-center justify-center gap-2 px-6 text-center"
        >
            <Icon name="heroicons:exclamation-triangle" size="24" class="text-lava" />
            <p class="text-sm font-medium text-white">Scene unavailable</p>
            <p class="text-dim text-xs">{{ sceneError }}</p>
        </div>
        <template v-else-if="scene && mountedItemId !== null">
            <ScenePlayer
                :key="mountedItemId"
                ref="playerRef"
                :scene-url="streamUrl"
                :transcode-url="transcodeUrl"
                :poster-url="posterUrl"
                :scene="scene"
                :autoplay="false"
                :loop="false"
                :default-volume="settingsStore.defaultVolume"
                :start-time="startTime"
                playlist-mode
                :has-next="canControl"
                :has-previous="false"
                keep-resume
                @ended="reportEnded"
                @next="skip"
            />
            <TogetherRoomReactionsOverlay :target="playerRef?.playerEl ?? null" />
        </template>

        <div
            v-if="onHold"
            class="pointer-events-none absolute top-3 left-1/2 z-20 -translate-x-1/2 rounded-full
                border border-white/10 bg-black/60 px-3 py-1 text-[11px] text-white
                backdrop-blur-md"
        >
            <Icon name="svg-spinners:90-ring-with-bg" size="11" class="mr-1 align-[-1px]" />
            Waiting for everyone...
        </div>
        <button
            v-if="autoplayBlocked"
            class="border-lava/40 bg-void/80 hover:bg-lava/20 absolute right-3 bottom-16 z-20 flex
                items-center gap-1.5 rounded-full border px-3 py-1.5 text-[11px] font-medium
                text-white backdrop-blur-md transition-colors"
            @click="unmute"
        >
            <Icon name="heroicons:speaker-x-mark" size="13" />
            Click to unmute
        </button>
    </div>
</template>
