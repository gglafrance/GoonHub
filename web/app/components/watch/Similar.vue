<script setup lang="ts">
import type { Marker } from '~/types/marker';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

// Scenes sharing the duplicate group (fingerprint match) of the watched scene
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);
const seekToTime = inject<(time: number) => void>('seekToTime');
const playMarker =
    inject<(marker: Pick<Marker, 'timestamp' | 'end_timestamp'>) => void>('playMarker');

// The group link opens the Duplicates page
const perms = useMyPermissions();
const canOpenGroup = computed(() => perms.has('duplicates:manage'));
const similar = computed(() => watchPageData?.similar.value ?? null);
const scenes = computed(() => similar.value?.scenes ?? []);
const currentSceneId = computed(() => similar.value?.scene_id ?? 0);
const loading = computed(() => watchPageData?.loading.related ?? false);

const statusLabel = computed(() => {
    switch (similar.value?.group_status) {
        case 'resolved':
            return 'Group resolved';
        case 'dismissed':
            return 'Group dismissed';
        default:
            return '';
    }
});

const groupLink = computed(() =>
    similar.value?.group_id
        ? duplicateGroupLink(similar.value.group_id, similar.value.group_status)
        : null,
);

// Plays the watched scene from where it matches (a point: no A/B loop)
const seekHere = (time: number) => {
    if (playMarker) playMarker({ timestamp: time, end_timestamp: null });
    else seekToTime?.(time);
    window.scrollTo({ top: 0, behavior: 'smooth' });
    document.getElementById('video-player')?.focus();
};
</script>

<template>
    <div class="space-y-3">
        <div v-if="loading && scenes.length === 0" class="text-dim py-4 text-center text-[11px]">
            Loading...
        </div>

        <div v-else-if="scenes.length === 0" class="text-dim py-4 text-center text-[11px]">
            No similar scenes
        </div>

        <template v-else>
            <div class="flex items-center justify-between gap-2 text-[11px]">
                <span class="text-dim">
                    {{ scenes.length }} {{ scenes.length === 1 ? 'scene matches' : 'scenes match' }}
                    this one by fingerprint
                    <span
                        v-if="statusLabel"
                        class="text-dim ml-1 rounded-full bg-white/5 px-1.5 py-0.5 text-[9px]
                            font-medium"
                    >
                        {{ statusLabel }}
                    </span>
                </span>
                <NuxtLink
                    v-if="canOpenGroup && groupLink"
                    :to="groupLink"
                    class="text-dim hover:text-lava inline-flex items-center gap-1
                        transition-colors"
                    title="Open this duplicate group on the Duplicates page"
                >
                    <Icon name="heroicons:square-2-stack" size="12" />
                    Duplicate group #{{ similar?.group_id }}
                </NuxtLink>
            </div>

            <WatchSimilarRow
                v-for="scene in scenes"
                :key="scene.id"
                :scene="scene"
                :current-scene-id="currentSceneId"
                @seek="seekHere"
            />
        </template>
    </div>
</template>
