<script setup lang="ts">
import type { DuplicateGroupMember } from '~/types/duplicates';

// Plays every video of a duplicate group side by side, each one starting at its matching
// segment, with shared controls (duplicateCompare.ts): play/pause, seek keeping every video on
// the same moment of its segment, one audio source at a time. Large groups are paged, only the
// shown page is loaded (metadata only until played).
const props = defineProps<{
    visible: boolean;
    members: DuplicateGroupMember[];
    // video the player was opened from: shown first and heard
    sceneId: number | null;
    // members unchecked as not duplicates (see duplicateEligibility.ts)
    unchecked: ReadonlySet<number>;
    // the group is unresolved: eligibility can be changed
    editable: boolean;
}>();

const emit = defineEmits<{
    close: [];
    toggleEligible: [sceneId: number];
}>();

const hideUnchecked = ref(false);
const page = ref(0);
const offset = ref(0);
const playing = ref(false);
const muted = ref(true);
const audioId = ref<number | null>(null);
// The seek bar is being dragged: the playing videos don't move it meanwhile
const scrubbing = ref(false);

// Video elements of the shown page, by scene ID (not reactive)
const videos = new Map<number, HTMLVideoElement>();

const shown = computed(() =>
    hideUnchecked.value
        ? props.members.filter((m) => !props.unchecked.has(m.scene_id))
        : props.members,
);
const pageCount = computed(() => comparePageCount(shown.value.length));
const pageMembers = computed(() => comparePageSlice(shown.value, page.value));
const range = computed(() => compareOffsetRange(pageMembers.value));
// First and last video shown (1-based)
const pageFrom = computed(() => Math.min(shown.value.length, page.value * COMPARE_PAGE_SIZE + 1));
const pageTo = computed(() => page.value * COMPARE_PAGE_SIZE + pageMembers.value.length);
const columnsClass = computed(
    () =>
        ({ 1: '', 2: 'md:grid-cols-2', 3: 'md:grid-cols-2 xl:grid-cols-3' })[
            compareGridColumns(pageMembers.value.length)
        ] ?? '',
);
const hasMissingSegment = computed(() => props.members.some((m) => m.match_start === undefined));

// The tile driving the shared position while playing: the heard one when shown, else the first
const masterId = computed(() => {
    const ids = pageMembers.value.map((m) => m.scene_id);
    return audioId.value !== null && ids.includes(audioId.value) ? audioId.value : ids[0];
});

const setVideo = (sceneId: number, el: unknown) => {
    if (el instanceof HTMLVideoElement) videos.set(sceneId, el);
    else videos.delete(sceneId);
};

const memberOf = (sceneId: number) => pageMembers.value.find((m) => m.scene_id === sceneId);

const applyAudio = () => {
    for (const [id, el] of videos) el.muted = muted.value || id !== audioId.value;
};

const seekAll = (value: number) => {
    offset.value = value;
    for (const [id, el] of videos) {
        const member = memberOf(id);
        if (member) el.currentTime = compareTileTime(member, value);
    }
};

const playAll = () => {
    playing.value = true;
    for (const el of videos.values()) el.play().catch(() => {});
};

const pauseAll = () => {
    playing.value = false;
    for (const el of videos.values()) el.pause();
};

const toggleMute = () => {
    muted.value = !muted.value;
    if (!muted.value && audioId.value === null) audioId.value = masterId.value ?? null;
    applyAudio();
};

const selectAudio = (sceneId: number) => {
    audioId.value = sceneId;
    muted.value = false;
    applyAudio();
};

const onLoaded = (member: DuplicateGroupMember) => {
    const el = videos.get(member.scene_id);
    if (!el) return;
    el.currentTime = compareTileTime(member, offset.value);
    el.muted = muted.value || member.scene_id !== audioId.value;
    if (playing.value) el.play().catch(() => {});
};

// The master tile moves the shared position; the others are pulled back when they drift by
// more than a second (decoding speeds differ)
const onTimeUpdate = (member: DuplicateGroupMember) => {
    if (scrubbing.value || member.scene_id !== masterId.value) return;
    const el = videos.get(member.scene_id);
    if (!el) return;
    offset.value = compareOffsetOf(member, el.currentTime);
    if (!playing.value || el.paused) return;
    for (const [id, other] of videos) {
        const m = id === member.scene_id ? undefined : memberOf(id);
        if (!m || other.paused || other.seeking) continue;
        const expected = compareTileTime(m, offset.value);
        if (Math.abs(other.currentTime - expected) > 1) other.currentTime = expected;
    }
};

// Unmuting a video with its own controls makes it the heard one
const onVolumeChange = (member: DuplicateGroupMember) => {
    const el = videos.get(member.scene_id);
    if (!el) return;
    const heard = !muted.value && audioId.value === member.scene_id;
    if (!el.muted && !heard) selectAudio(member.scene_id);
    else if (el.muted && heard) muted.value = true;
};

const onSeek = (value: number) => {
    scrubbing.value = true;
    seekAll(value);
};

const goToPage = (p: number) => {
    page.value = Math.min(Math.max(0, p), pageCount.value - 1);
};

// Opening: the page of the clicked video, heard first (muted until the sound is turned on)
watch(
    () => [props.visible, props.sceneId] as const,
    ([visible]) => {
        if (!visible) {
            pauseAll();
            videos.clear();
            return;
        }
        hideUnchecked.value = false;
        offset.value = 0;
        playing.value = false;
        muted.value = true;
        audioId.value = props.sceneId;
        page.value = comparePageOf(props.members.findIndex((m) => m.scene_id === props.sceneId));
    },
    { immediate: true },
);

watch(pageCount, () => goToPage(page.value));

const handleEscape = (e: KeyboardEvent) => {
    if (props.visible && e.key === 'Escape') emit('close');
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible && members.length"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4
                backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div
                class="border-border bg-panel flex max-h-full w-full max-w-[min(96vw,110rem)]
                    flex-col rounded-xl border p-4 shadow-2xl"
            >
                <div class="mb-3 flex items-center justify-between gap-3">
                    <h3 class="text-sm font-medium text-white">
                        Compare duplicates
                        <span class="text-dim text-[11px] font-normal">
                            {{ members.length }} videos{{
                                unchecked.size ? `, ${unchecked.size} unchecked` : ''
                            }}
                        </span>
                    </h3>
                    <button class="text-dim p-1 hover:text-white" @click="emit('close')">
                        <Icon name="heroicons:x-mark" size="16" />
                    </button>
                </div>

                <DuplicatesPlayerToolbar
                    v-model:hide-unchecked="hideUnchecked"
                    class="mb-3"
                    :playing="playing"
                    :muted="muted"
                    :offset="offset"
                    :range="range"
                    :page="page"
                    :page-count="pageCount"
                    :from="pageFrom"
                    :to="pageTo"
                    :total="shown.length"
                    :editable="editable"
                    :unchecked-count="unchecked.size"
                    @play="playAll"
                    @pause="pauseAll"
                    @jump="seekAll(0)"
                    @toggle-mute="toggleMute"
                    @seek="onSeek"
                    @seek-end="scrubbing = false"
                    @page="goToPage"
                />

                <div class="min-h-0 overflow-y-auto">
                    <div v-if="pageMembers.length" class="grid gap-3" :class="columnsClass">
                        <div
                            v-for="member in pageMembers"
                            :key="member.scene_id"
                            class="min-w-0 rounded-lg border p-1.5 transition-colors"
                            :class="[
                                !muted && audioId === member.scene_id
                                    ? 'border-lava/50'
                                    : 'border-transparent',
                                unchecked.has(member.scene_id) ? 'opacity-50' : '',
                            ]"
                        >
                            <video
                                :ref="(el) => setVideo(member.scene_id, el)"
                                :src="`/api/v1/scenes/${member.scene_id}/stream`"
                                controls
                                muted
                                playsinline
                                preload="metadata"
                                class="aspect-video w-full rounded-md bg-black"
                                @loadedmetadata="onLoaded(member)"
                                @timeupdate="onTimeUpdate(member)"
                                @volumechange="onVolumeChange(member)"
                            />
                            <DuplicatesPlayerTileInfo
                                :member="member"
                                :audio="!muted && audioId === member.scene_id"
                                :editable="editable"
                                :eligible="!unchecked.has(member.scene_id)"
                                @select-audio="selectAudio(member.scene_id)"
                                @toggle-eligible="emit('toggleEligible', member.scene_id)"
                            />
                        </div>
                    </div>
                    <p v-else class="text-dim py-10 text-center text-xs">
                        Every video is unchecked.
                    </p>
                    <p v-if="hasMissingSegment" class="text-dim mt-3 text-[10px]">
                        Videos without a matching segment start at the beginning. Segments are
                        recorded for groups detected from now on; re-run the fingerprint of these
                        scenes to get them for older groups.
                    </p>
                </div>
            </div>
        </div>
    </Teleport>
</template>
