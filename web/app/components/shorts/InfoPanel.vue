<script setup lang="ts">
import type { Scene } from '~/types/scene';
import { formatShortClock } from '~/composables/useShortEditor';

// Everything about the short in view: file details, studio, actors and tags.
// Slides over the right of the player and follows the active short.
const props = defineProps<{
    sceneId: number;
}>();

const emit = defineEmits<{
    close: [];
}>();

interface NamedRef {
    id: number;
    uuid?: string;
    name: string;
    color?: string;
}

interface SceneInfo {
    scene: Scene;
    studio: NamedRef | null;
    actors: NamedRef[];
    tags: NamedRef[];
}

const { fetchScene } = useApiScenes();
const { fetchSceneTags } = useApiTags();
const { fetchSceneActors } = useApiActors();
const { fetchSceneStudio } = useApiStudios();
const { formatSize, formatBitRate, formatFrameRate, formatDuration } = useFormatter();

// Kept for the visit so scrolling back and forth does not refetch.
const cache = new Map<number, SceneInfo>();
const info = ref<SceneInfo | null>(null);
const loading = ref(false);
const error = ref('');

const load = async (id: number) => {
    const cached = cache.get(id);
    if (cached) {
        info.value = cached;
        error.value = '';
        return;
    }
    loading.value = true;
    error.value = '';
    try {
        const [scene, tags, actors, studio] = await Promise.all([
            fetchScene(id),
            fetchSceneTags(id).catch(() => ({ data: [] })),
            fetchSceneActors(id).catch(() => ({ data: [] })),
            fetchSceneStudio(id).catch(() => ({ data: null })),
        ]);
        const loaded: SceneInfo = {
            scene: scene as Scene,
            tags: (tags?.data ?? []) as NamedRef[],
            actors: (actors?.data ?? []) as NamedRef[],
            studio: (studio?.data ?? null) as NamedRef | null,
        };
        cache.set(id, loaded);
        if (id === props.sceneId) info.value = loaded;
    } catch (e: unknown) {
        if (id === props.sceneId) {
            error.value = e instanceof Error ? e.message : 'Failed to load video info';
        }
    } finally {
        if (id === props.sceneId) loading.value = false;
    }
};

watch(() => props.sceneId, load, { immediate: true });

const resolutionLabel = (w?: number, h?: number) => {
    if (!w || !h) return '';
    const short = Math.min(w, h);
    const name =
        short >= 2160
            ? '4K'
            : short >= 1440
              ? '1440p'
              : short >= 1080
                ? '1080p'
                : short >= 720
                  ? '720p'
                  : short >= 480
                    ? '480p'
                    : `${short}p`;
    return `${w} x ${h} (${name}${h > w ? ', vertical' : ''})`;
};

const formatDate = (v?: string | null) => {
    if (!v) return '';
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? '' : d.toLocaleString();
};

const details = computed(() => {
    const s = info.value?.scene;
    if (!s) return [];
    const rows: [string, string][] = [
        ['Duration', s.duration ? formatDuration(s.duration) : ''],
        ['Resolution', resolutionLabel(s.width, s.height)],
        ['Frame rate', s.frame_rate ? formatFrameRate(s.frame_rate) : ''],
        ['Bit rate', s.bit_rate ? formatBitRate(s.bit_rate) : ''],
        ['Video codec', s.video_codec ?? ''],
        ['Audio codec', s.audio_codec ?? ''],
        ['File size', s.size ? formatSize(s.size) : ''],
        ['Views', String(s.view_count ?? 0)],
        ['Released', s.release_date ? (s.release_date.split('T')[0] ?? '') : ''],
        ['Added', formatDate(s.created_at)],
        ['File created', formatDate(s.file_created_at)],
        ['Origin', s.origin ?? ''],
        ['Type', s.type ?? ''],
        ['File name', s.original_filename ?? ''],
        ['Path', s.stored_path ?? ''],
    ];
    return rows.filter(([, v]) => v);
});

const source = computed(() => {
    const s = info.value?.scene;
    if (!s?.source_scene_id || s.source_start == null || s.source_end == null) return null;
    return {
        id: s.source_scene_id,
        range: `${formatShortClock(s.source_start)}-${formatShortClock(s.source_end)}`,
    };
});
</script>

<template>
    <aside
        class="border-border absolute inset-y-0 right-0 z-20 flex w-full flex-col border-l
            bg-black/85 backdrop-blur-xl sm:w-80"
        @click.stop
        @wheel.stop
    >
        <div class="border-border flex shrink-0 items-center gap-2 border-b px-4 py-3">
            <Icon name="heroicons:information-circle" size="16" class="text-lava" />
            <h2 class="mr-auto text-xs font-semibold tracking-wide text-white uppercase">
                Video info
            </h2>
            <button
                class="text-dim flex h-7 w-7 items-center justify-center rounded-md
                    transition-colors hover:bg-white/10 hover:text-white"
                title="Close (I)"
                @click="emit('close')"
            >
                <Icon name="heroicons:x-mark" size="16" />
            </button>
        </div>

        <div class="min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain px-4 py-4">
            <div v-if="loading && !info" class="flex justify-center py-10">
                <LoadingSpinner />
            </div>
            <p v-else-if="error" class="text-lava text-xs">{{ error }}</p>

            <template v-if="info">
                <div class="space-y-1.5">
                    <NuxtLink
                        :to="`/watch/${info.scene.id}`"
                        class="block text-sm leading-snug font-semibold text-white hover:underline"
                    >
                        {{ info.scene.title }}
                    </NuxtLink>
                    <p
                        v-if="info.scene.description"
                        class="text-dim text-xs leading-relaxed whitespace-pre-line"
                    >
                        {{ info.scene.description }}
                    </p>
                    <NuxtLink
                        v-if="source"
                        :to="`/watch/${source.id}`"
                        class="text-dim hover:text-lava flex items-center gap-1 text-[11px]
                            transition-colors"
                    >
                        <Icon name="heroicons:scissors" size="12" />
                        Created from scene {{ source.id }}, {{ source.range }}
                    </NuxtLink>
                </div>

                <section v-if="info.studio" class="space-y-1.5">
                    <h3 class="info-heading">Studio</h3>
                    <NuxtLink
                        :to="info.studio.uuid ? `/studios/${info.studio.uuid}` : '/studios'"
                        class="info-chip"
                    >
                        <Icon name="heroicons:building-office-2" size="12" />
                        {{ info.studio.name }}
                    </NuxtLink>
                </section>

                <section class="space-y-1.5">
                    <h3 class="info-heading">Actors</h3>
                    <div v-if="info.actors.length" class="flex flex-wrap gap-1.5">
                        <NuxtLink
                            v-for="a in info.actors"
                            :key="a.id"
                            :to="a.uuid ? `/actors/${a.uuid}` : '/actors'"
                            class="info-chip"
                        >
                            <Icon name="heroicons:user" size="12" />
                            {{ a.name }}
                        </NuxtLink>
                    </div>
                    <p v-else class="text-dim text-[11px]">None</p>
                </section>

                <section class="space-y-1.5">
                    <h3 class="info-heading">Tags</h3>
                    <div v-if="info.tags.length" class="flex flex-wrap gap-1.5">
                        <NuxtLink
                            v-for="t in info.tags"
                            :key="t.id"
                            :to="{ path: '/search', query: { tags: t.name } }"
                            class="info-chip"
                        >
                            <span
                                class="h-1.5 w-1.5 rounded-full"
                                :style="{ background: t.color || '#6B7280' }"
                            />
                            {{ t.name }}
                        </NuxtLink>
                    </div>
                    <p v-else class="text-dim text-[11px]">None</p>
                </section>

                <section class="space-y-1.5">
                    <h3 class="info-heading">Details</h3>
                    <dl class="space-y-1.5">
                        <div
                            v-for="[label, value] in details"
                            :key="label"
                            class="grid grid-cols-[6.5rem_1fr] gap-2 text-[11px]"
                        >
                            <dt class="text-dim">{{ label }}</dt>
                            <dd class="font-mono break-all text-white/90">{{ value }}</dd>
                        </div>
                    </dl>
                </section>
            </template>
        </div>
    </aside>
</template>

<style scoped>
.info-heading {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: rgba(255, 255, 255, 0.5);
}

.info-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    border-radius: 9999px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    background: rgba(255, 255, 255, 0.04);
    padding: 0.2rem 0.6rem;
    font-size: 11px;
    color: #fff;
    transition:
        border-color 0.15s ease,
        background 0.15s ease;
}

.info-chip:hover {
    border-color: rgba(255, 77, 77, 0.45);
    background: rgba(255, 77, 77, 0.1);
}
</style>
