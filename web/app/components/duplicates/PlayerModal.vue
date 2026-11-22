<script setup lang="ts">
import type { DuplicateGroupMember } from '~/types/duplicates';

// Plays a duplicate group member, side by side with the scene it was matched against, both
// positioned on the matching segment.
const props = defineProps<{
    visible: boolean;
    members: DuplicateGroupMember[];
    sceneId: number | null;
}>();

const emit = defineEmits<{
    close: [];
}>();

const { formatDuration } = useFormatter();

const primary = computed(() => props.members.find((m) => m.scene_id === props.sceneId) ?? null);

// The other side: the matched reference scene, else the group's best, else another member
const secondary = computed(() => {
    const p = primary.value;
    if (!p) return null;
    const others = props.members.filter((m) => m.scene_id !== p.scene_id);
    return (
        others.find((m) => m.scene_id === p.ref_scene_id) ??
        others.find((m) => m.is_best) ??
        others[0] ??
        null
    );
});

interface Side {
    member: DuplicateGroupMember;
    start?: number;
    end?: number;
}

// Matching segment of each side: the primary's own segment, and on the other side the
// reference segment when it is the matched scene (or that scene's own segment otherwise).
const sides = computed<Side[]>(() => {
    const p = primary.value;
    if (!p) return [];
    const result: Side[] = [{ member: p, start: p.match_start, end: p.match_end }];
    const s = secondary.value;
    if (s) {
        const isRef = s.scene_id === p.ref_scene_id;
        result.push({
            member: s,
            start: isRef ? p.ref_start : s.match_start,
            end: isRef ? p.ref_end : s.match_end,
        });
    }
    return result;
});

const videos = ref<HTMLVideoElement[]>([]);

const seekToMatch = () => {
    sides.value.forEach((side, i) => {
        const el = videos.value[i];
        if (el && side.start !== undefined) el.currentTime = side.start;
    });
};

const playBoth = () => {
    videos.value.forEach((el) => el.play().catch(() => {}));
};

const pauseBoth = () => {
    videos.value.forEach((el) => el.pause());
};

const onLoaded = (index: number) => {
    const side = sides.value[index];
    const el = videos.value[index];
    if (el && side?.start !== undefined) el.currentTime = side.start;
};

const segmentStyle = (side: Side) => {
    const duration = side.member.duration;
    if (side.start === undefined || side.end === undefined || duration <= 0) return null;
    const left = Math.min(100, (side.start / duration) * 100);
    const width = Math.max(0.5, Math.min(100 - left, ((side.end - side.start) / duration) * 100));
    return { left: `${left}%`, width: `${width}%` };
};

const handleEscape = (e: KeyboardEvent) => {
    if (props.visible && e.key === 'Escape') emit('close');
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && primary"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4
                backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="border-border bg-panel w-full max-w-6xl rounded-xl border p-4 shadow-2xl">
                <div class="mb-3 flex items-center justify-between">
                    <h3 class="text-sm font-medium text-white">Compare duplicates</h3>
                    <div class="flex items-center gap-2">
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                items-center gap-1 rounded-md border px-2 py-1 text-[11px]
                                transition-all"
                            title="Seek both players to the matching segment"
                            @click="seekToMatch"
                        >
                            <Icon name="heroicons:arrows-pointing-in" size="12" />
                            Jump to match
                        </button>
                        <button
                            class="border-border text-dim flex items-center gap-1 rounded-md border
                                px-2 py-1 text-[11px] transition-all hover:text-white"
                            @click="playBoth"
                        >
                            <Icon name="heroicons:play" size="12" />
                            Play both
                        </button>
                        <button
                            class="border-border text-dim flex items-center gap-1 rounded-md border
                                px-2 py-1 text-[11px] transition-all hover:text-white"
                            @click="pauseBoth"
                        >
                            <Icon name="heroicons:pause" size="12" />
                            Pause
                        </button>
                        <button class="text-dim p-1 hover:text-white" @click="emit('close')">
                            <Icon name="heroicons:x-mark" size="16" />
                        </button>
                    </div>
                </div>

                <div class="grid gap-3" :class="sides.length > 1 ? 'md:grid-cols-2' : ''">
                    <div v-for="(side, index) in sides" :key="side.member.scene_id">
                        <video
                            :ref="(el) => (videos[index] = el as HTMLVideoElement)"
                            :src="`/api/v1/scenes/${side.member.scene_id}/stream`"
                            controls
                            muted
                            preload="metadata"
                            class="aspect-video w-full rounded-lg bg-black"
                            @loadedmetadata="onLoaded(index)"
                        />
                        <!-- Matching segment on the timeline -->
                        <div class="relative mt-1.5 h-1.5 rounded-full bg-white/10">
                            <div
                                v-if="segmentStyle(side)"
                                class="bg-lava absolute h-full rounded-full"
                                :style="segmentStyle(side)!"
                            />
                        </div>
                        <div class="mt-1.5 flex items-start justify-between gap-2 text-[11px]">
                            <div class="min-w-0">
                                <div class="flex min-w-0 items-center gap-1.5">
                                    <DuplicatesRelationBadge :member="side.member" />
                                    <span
                                        class="truncate text-white"
                                        :title="side.member.file_name"
                                    >
                                        #{{ side.member.scene_id }} {{ side.member.title }}
                                    </span>
                                </div>
                                <div class="text-dim font-mono text-[10px]">
                                    {{ side.member.width }}x{{ side.member.height }} ·
                                    {{ formatDuration(side.member.duration) }} ·
                                    {{ side.member.video_codec }}
                                </div>
                            </div>
                            <span
                                v-if="side.start !== undefined && side.end !== undefined"
                                class="bg-lava/10 text-lava shrink-0 rounded px-1.5 py-0.5 font-mono
                                    text-[10px]"
                                title="Matching segment"
                            >
                                {{ formatDuration(side.start) }}-{{ formatDuration(side.end) }}
                            </span>
                            <span v-else class="text-dim shrink-0 text-[10px]">
                                No segment recorded
                            </span>
                        </div>
                    </div>
                </div>
                <p v-if="primary.match_start === undefined" class="text-dim mt-3 text-[10px]">
                    Matching segments are recorded for groups detected from now on; re-run the
                    fingerprint of these scenes to get them for older groups.
                </p>
            </div>
        </div>
    </Teleport>
</template>
