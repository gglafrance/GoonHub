<script setup lang="ts">
import type { DuplicateGroupMember } from '~/types/duplicates';

// Details under one video of the compare player: matching segment on the timeline, eligible
// checkbox, relation, title, best marker, audio source, then resolution, duration and size.
const props = defineProps<{
    member: DuplicateGroupMember;
    // this tile is the one heard (shared mute off)
    audio: boolean;
    // the eligible checkbox is shown (unresolved group)
    editable: boolean;
    eligible: boolean;
}>();

const emit = defineEmits<{
    selectAudio: [];
    toggleEligible: [];
}>();

const { formatDuration, formatSize } = useFormatter();

const segment = computed(() => compareSegmentStyle(props.member));
const hasSegment = computed(
    () => props.member.match_start !== undefined && props.member.match_end !== undefined,
);
</script>

<template>
    <div>
        <!-- Matching segment on the timeline -->
        <div class="relative mt-1.5 h-1.5 rounded-full bg-white/10">
            <div v-if="segment" class="bg-lava absolute h-full rounded-full" :style="segment" />
        </div>
        <div class="mt-1.5 flex items-start justify-between gap-2 text-[11px]">
            <div class="min-w-0">
                <div class="flex min-w-0 items-center gap-1.5">
                    <input
                        v-if="editable"
                        type="checkbox"
                        class="accent-lava h-3 w-3 shrink-0 cursor-pointer"
                        :checked="eligible"
                        :title="
                            eligible
                                ? 'Eligible duplicate: uncheck if this video is not a duplicate'
                                : 'Not a duplicate: removed from the group before resolving or trashing'
                        "
                        @change="emit('toggleEligible')"
                    />
                    <DuplicatesRelationBadge :member="member" />
                    <span
                        class="truncate"
                        :class="eligible ? 'text-white' : 'text-dim line-through'"
                        :title="member.file_name"
                    >
                        #{{ member.scene_id }} {{ member.title }}
                    </span>
                    <span
                        v-if="member.is_best"
                        class="bg-emerald/20 text-emerald flex shrink-0 items-center gap-0.5
                            rounded-full px-1.5 py-0.5 text-[9px] font-semibold"
                        title="Best variant, kept at resolution"
                    >
                        <Icon name="heroicons:check" size="10" />
                        Best
                    </span>
                    <span
                        v-if="member.is_trashed"
                        class="text-dim shrink-0 rounded-full bg-white/5 px-1.5 py-0.5 text-[9px]"
                    >
                        Trashed
                    </span>
                </div>
                <div class="text-dim truncate font-mono text-[10px]">
                    {{ member.width }}x{{ member.height }} · {{ formatDuration(member.duration) }} ·
                    {{ member.size > 0 ? formatSize(member.size) : 'size unknown' }} ·
                    {{ member.video_codec }}
                </div>
            </div>
            <div class="flex shrink-0 items-center gap-1.5">
                <span
                    v-if="hasSegment"
                    class="bg-lava/10 text-lava rounded px-1.5 py-0.5 font-mono text-[10px]"
                    title="Matching segment"
                >
                    {{ formatDuration(member.match_start!) }}-{{
                        formatDuration(member.match_end!)
                    }}
                </span>
                <span v-else class="text-dim text-[10px]">No segment recorded</span>
                <button
                    class="flex h-5 w-5 items-center justify-center rounded transition-colors"
                    :class="audio ? 'bg-lava/20 text-lava' : 'text-dim hover:text-white'"
                    :title="audio ? 'Audio from this video' : 'Hear this video'"
                    @click="emit('selectAudio')"
                >
                    <Icon
                        :name="audio ? 'heroicons:speaker-wave' : 'heroicons:speaker-x-mark'"
                        size="12"
                    />
                </button>
            </div>
        </div>
    </div>
</template>
