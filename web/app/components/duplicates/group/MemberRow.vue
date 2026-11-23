<script setup lang="ts">
import type { DuplicateGroupMember } from '~/types/duplicates';

// One scene of a duplicate group in the comparison table.
const props = defineProps<{
    member: DuplicateGroupMember;
    unresolved: boolean;
    // checked as a duplicate (unchecked: removed from the group before resolving or trashing)
    eligible: boolean;
}>();

const emit = defineEmits<{
    toggleEligible: [];
    setBest: [];
    play: [];
    cancelDeletion: [];
}>();

const { formatDuration, formatSize, formatBitRate } = useFormatter();

const hasSegment = computed(
    () => props.member.match_start !== undefined && props.member.match_end !== undefined,
);
</script>

<template>
    <tr
        class="border-border border-b transition-colors last:border-b-0"
        :class="{
            'bg-emerald/5': member.is_best,
            'opacity-60': member.is_trashed,
            'opacity-50': !eligible,
            'hover:bg-white/2': !member.is_best && !member.is_trashed,
        }"
    >
        <td v-if="unresolved" class="py-2.5 pl-4">
            <input
                type="checkbox"
                class="accent-lava h-3.5 w-3.5 cursor-pointer"
                :checked="eligible"
                :title="
                    eligible
                        ? 'Eligible duplicate: uncheck if this video is not a duplicate'
                        : 'Not a duplicate: removed from the group before resolving or trashing'
                "
                @click.stop
                @change="emit('toggleEligible')"
            />
        </td>
        <td class="px-4 py-2.5">
            <div class="flex items-center gap-2">
                <button
                    class="group/play relative h-8 w-12 shrink-0 overflow-hidden rounded
                        bg-black/50"
                    :class="{ grayscale: member.is_trashed }"
                    title="Play and compare"
                    @click.stop="emit('play')"
                >
                    <img
                        :src="`/thumbnails/${member.scene_id}?size=sm`"
                        class="h-full w-full object-cover"
                        :alt="member.title"
                    />
                    <span
                        class="bg-lava/40 absolute inset-0 flex items-center justify-center
                            opacity-0 transition-opacity group-hover/play:opacity-100"
                    >
                        <Icon name="heroicons:play-solid" size="14" class="text-white" />
                    </span>
                </button>
                <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                        <NuxtLink
                            :to="`/watch/${member.scene_id}`"
                            class="max-w-56 truncate font-medium transition-colors"
                            :class="[
                                member.is_trashed ? 'text-dim' : 'hover:text-lava text-white',
                                { 'line-through': !eligible },
                            ]"
                            @click.stop
                        >
                            {{ member.title }}
                        </NuxtLink>
                        <span
                            v-if="member.is_trashed"
                            class="text-dim rounded-full bg-white/5 px-1.5 py-0.5 text-[9px]
                                font-medium"
                        >
                            Trashed
                        </span>
                    </div>
                    <div
                        class="text-dim max-w-64 truncate font-mono text-[10px]"
                        :title="member.file_name"
                    >
                        {{ member.file_name }}
                    </div>
                </div>
            </div>
        </td>
        <td class="px-3 py-2.5 text-white">{{ member.width }}x{{ member.height }}</td>
        <td class="px-3 py-2.5 text-white">{{ formatDuration(member.duration) }}</td>
        <td class="px-3 py-2.5 text-white">{{ member.video_codec }}</td>
        <td class="px-3 py-2.5 text-white">{{ formatBitRate(member.bit_rate) }}</td>
        <td class="px-3 py-2.5 text-white">{{ formatSize(member.size) }}</td>
        <td class="px-3 py-2.5">
            <div class="flex items-center gap-1.5">
                <DuplicatesRelationBadge :member="member" />
                <span class="text-dim">{{ (member.confidence_score * 100).toFixed(0) }}%</span>
            </div>
            <button
                v-if="hasSegment"
                class="text-lava/80 hover:text-lava font-mono text-[10px] transition-colors"
                :title="
                    member.ref_scene_id
                        ? `Matches scene #${member.ref_scene_id} at ${formatDuration(member.ref_start ?? 0)}-${formatDuration(member.ref_end ?? 0)}`
                        : 'Matching segment'
                "
                @click.stop="emit('play')"
            >
                {{ formatDuration(member.match_start!) }}-{{ formatDuration(member.match_end!) }}
            </button>
        </td>
        <td class="px-3 py-2.5 text-center">
            <button
                v-if="unresolved"
                class="inline-flex h-5 w-5 items-center justify-center rounded-full border
                    transition-all"
                :class="
                    member.is_best
                        ? 'border-emerald bg-emerald/20 text-emerald'
                        : 'text-dim hover:border-emerald/50 border-white/20'
                "
                @click.stop="emit('setBest')"
            >
                <Icon v-if="member.is_best" name="heroicons:check" size="12" />
            </button>
            <Icon
                v-else-if="member.is_best"
                name="heroicons:check-circle-solid"
                size="16"
                class="text-emerald"
            />
            <div v-else-if="member.delete_after" class="flex flex-col items-center gap-0.5">
                <span class="text-lava text-[9px]" title="Scheduled permanent deletion">
                    Deleted <NuxtTime :datetime="member.delete_after" relative />
                </span>
                <button
                    class="text-dim text-[9px] underline transition-colors hover:text-white"
                    @click.stop="emit('cancelDeletion')"
                >
                    Cancel deletion
                </button>
            </div>
        </td>
    </tr>
</template>
