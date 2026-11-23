<script setup lang="ts">
import type { DuplicateGroup } from '~/types/duplicates';

// A duplicate group: selectable header, comparison table and actions.
const props = defineProps<{
    group: DuplicateGroup;
    expanded: boolean;
    selected: boolean;
    mergeMetadata: boolean;
    // members unchecked as not duplicates (see duplicateEligibility.ts)
    unchecked: ReadonlySet<number>;
}>();

const emit = defineEmits<{
    toggle: [];
    toggleSelect: [];
    setBest: [sceneId: number];
    autoSelect: [];
    resolve: [];
    dismiss: [];
    trashAll: [];
    play: [sceneId: number];
    cancelDeletion: [sceneId: number];
    toggleEligible: [sceneId: number];
    setAllEligible: [eligible: boolean];
    excludeUnchecked: [];
    'update:mergeMetadata': [value: boolean];
}>();

const { kindLabel, kindTitle, kindClass } = useDuplicateRelation();

const unresolved = computed(() => props.group.status === 'unresolved');
// Only groups of identical scenes are handled by the automatic actions
const identical = computed(() => props.group.kind === 'identical');
const pendingDeletions = computed(() => props.group.members.filter((m) => m.delete_after).length);

// Eligible members: the unchecked ones are removed from the group before resolve / trash all
const uncheckedCount = computed(
    () => props.group.members.filter((m) => props.unchecked.has(m.scene_id)).length,
);
const eligibleCount = computed(() => props.group.members.length - uncheckedCount.value);
const resolveBlocker = computed(() => {
    const best = props.group.best_scene_id;
    if (!best) return 'Select the scene to keep first';
    if (props.unchecked.has(best))
        return 'The scene to keep is unchecked: check it or pick another one';
    if (uncheckedCount.value && eligibleCount.value < 2)
        return 'Fewer than 2 eligible videos: remove the unchecked ones from the group instead';
    return null;
});
</script>

<template>
    <div
        class="border-border overflow-hidden rounded-lg border bg-white/2"
        :class="{ 'ring-lava/40 ring-1': selected }"
    >
        <!-- Header -->
        <div class="flex items-center gap-3 p-4 transition-colors hover:bg-white/2">
            <input
                v-if="unresolved"
                type="checkbox"
                class="accent-lava h-3.5 w-3.5 shrink-0 cursor-pointer"
                :checked="selected"
                title="Select for bulk actions"
                @change="emit('toggleSelect')"
            />
            <button
                class="flex min-w-0 flex-1 items-center justify-between text-left"
                @click="emit('toggle')"
            >
                <div class="flex items-center gap-3">
                    <div class="flex -space-x-2">
                        <div
                            v-for="(member, idx) in group.members.slice(0, 3)"
                            :key="member.scene_id"
                            class="border-void h-10 w-16 overflow-hidden rounded border-2
                                bg-black/50"
                            :style="{ zIndex: 3 - idx }"
                        >
                            <img
                                v-if="member.thumbnail_path"
                                :src="`/thumbnails/${member.scene_id}?size=sm`"
                                class="h-full w-full object-cover"
                                :alt="member.title"
                            />
                        </div>
                        <div
                            v-if="group.members.length > 3"
                            class="border-void bg-panel text-dim flex h-10 w-10 items-center
                                justify-center rounded border-2 text-[10px] font-medium"
                        >
                            +{{ group.members.length - 3 }}
                        </div>
                    </div>
                    <div>
                        <div class="flex items-center gap-2">
                            <span class="text-sm font-medium text-white">
                                {{ group.scene_count }} scenes
                            </span>
                            <span
                                class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold
                                    uppercase"
                                :class="{
                                    'bg-lava/20 text-lava': group.status === 'unresolved',
                                    'bg-emerald/20 text-emerald': group.status === 'resolved',
                                    'text-dim bg-white/10': group.status === 'dismissed',
                                }"
                            >
                                {{ group.status }}
                            </span>
                            <span
                                v-if="group.members[0]"
                                class="rounded-full bg-white/5 px-1.5 py-0.5 text-[9px] font-medium"
                                :class="
                                    group.members[0].match_type === 'audio'
                                        ? 'text-blue-400'
                                        : 'text-purple-400'
                                "
                            >
                                {{ group.members[0].match_type }}
                            </span>
                            <span
                                class="rounded-full px-1.5 py-0.5 text-[9px] font-semibold"
                                :class="kindClass(group.kind)"
                                :title="kindTitle(group.kind)"
                            >
                                {{ kindLabel(group.kind) }}
                            </span>
                            <span
                                v-if="pendingDeletions"
                                class="bg-lava/10 text-lava rounded-full px-1.5 py-0.5 text-[9px]"
                            >
                                {{ pendingDeletions }} pending deletion
                            </span>
                        </div>
                        <div class="text-dim mt-0.5 text-[11px]">
                            <UiFormattedDate :datetime="group.created_at" />
                        </div>
                    </div>
                </div>
                <Icon
                    name="heroicons:chevron-down"
                    size="16"
                    class="text-dim transition-transform"
                    :class="{ 'rotate-180': expanded }"
                />
            </button>
        </div>

        <!-- Detail -->
        <div v-if="expanded" class="border-border border-t">
            <div class="overflow-x-auto">
                <table class="w-full text-[11px]">
                    <thead>
                        <tr class="border-border border-b bg-white/2">
                            <th v-if="unresolved" class="py-2 pl-4 text-left">
                                <input
                                    type="checkbox"
                                    class="accent-lava h-3.5 w-3.5 cursor-pointer"
                                    :checked="uncheckedCount === 0"
                                    :indeterminate="uncheckedCount > 0 && eligibleCount > 0"
                                    title="Eligible duplicates: uncheck the videos that are not duplicates"
                                    @change="emit('setAllEligible', uncheckedCount > 0)"
                                />
                            </th>
                            <th class="text-dim px-4 py-2 text-left font-medium">Scene</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Resolution</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Duration</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Codec</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Bitrate</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Size</th>
                            <th class="text-dim px-3 py-2 text-left font-medium">Match</th>
                            <th class="text-dim px-3 py-2 text-center font-medium">Best</th>
                        </tr>
                    </thead>
                    <tbody>
                        <DuplicatesGroupMemberRow
                            v-for="member in group.members"
                            :key="member.scene_id"
                            :member="member"
                            :unresolved="unresolved"
                            :eligible="!unchecked.has(member.scene_id)"
                            @toggle-eligible="emit('toggleEligible', member.scene_id)"
                            @set-best="emit('setBest', member.scene_id)"
                            @play="emit('play', member.scene_id)"
                            @cancel-deletion="emit('cancelDeletion', member.scene_id)"
                        />
                    </tbody>
                </table>
            </div>

            <div
                v-if="unresolved && uncheckedCount"
                class="border-border flex flex-wrap items-center justify-between gap-2 border-t px-4
                    py-2.5 text-[11px]"
            >
                <span class="text-dim">
                    <span class="text-white">{{ uncheckedCount }}</span>
                    unchecked video{{ uncheckedCount === 1 ? '' : 's' }}: not duplicates of the
                    others. Resolve and Trash all remove them from the group first and never touch
                    them.
                </span>
                <div class="flex items-center gap-2">
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="emit('setAllEligible', true)"
                    >
                        Check all
                    </button>
                    <button
                        class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                            items-center gap-1 rounded-md border px-2.5 py-1 font-medium
                            transition-all disabled:pointer-events-none disabled:opacity-40"
                        :disabled="eligibleCount === 0"
                        :title="
                            eligibleCount === 0
                                ? 'Keep at least one video in the group, or dismiss it'
                                : 'Remove the unchecked videos from this group and remember they are not duplicates of the others (nothing is trashed)'
                        "
                        @click="emit('excludeUnchecked')"
                    >
                        <Icon name="heroicons:minus-circle" size="12" />
                        Remove unchecked from group
                    </button>
                </div>
            </div>
            <div
                v-if="unresolved && !identical"
                class="border-border text-dim flex items-start gap-1.5 border-t px-4 pt-3
                    text-[11px]"
            >
                <Icon
                    name="heroicons:shield-exclamation"
                    size="14"
                    class="text-lava mt-px shrink-0"
                />
                <span>
                    Not all scenes are identical (a clip and the longer scene containing it, a
                    partial or an unverified match), so auto-select and auto-resolve skip this
                    group. Resolving it keeps the selected scene and moves every other scene to the
                    trash.
                </span>
            </div>
            <div
                v-if="unresolved"
                class="border-border flex flex-wrap items-center justify-between gap-2 p-4"
                :class="{ 'border-t': identical }"
            >
                <label
                    class="text-dim flex items-center gap-2 text-[11px]"
                    title="Markers are moved to the kept scene, shifted to where the same moment is in it; markers outside the matching part stay on the trashed scene"
                >
                    <input
                        :checked="mergeMetadata"
                        type="checkbox"
                        class="accent-lava rounded"
                        @change="
                            emit(
                                'update:mergeMetadata',
                                ($event.target as HTMLInputElement).checked,
                            )
                        "
                    />
                    Merge metadata (tags, actors, markers, history)
                </label>
                <div class="flex gap-2">
                    <button
                        class="border-border text-dim hover:border-emerald/40 hover:text-emerald
                            flex items-center gap-1 rounded-md border px-3 py-1.5 text-[11px]
                            font-medium transition-all disabled:pointer-events-none
                            disabled:opacity-40"
                        :disabled="!identical"
                        :title="
                            identical
                                ? 'Select the best variant with the configured criteria'
                                : 'Only for groups of identical scenes: pick the scene to keep by hand'
                        "
                        @click="emit('autoSelect')"
                    >
                        <Icon name="heroicons:sparkles" size="12" />
                        Auto-select best
                    </button>
                    <button
                        class="border-border text-dim flex items-center gap-1 rounded-md border px-3
                            py-1.5 text-[11px] font-medium transition-all hover:border-red-500/40
                            hover:text-red-400 disabled:pointer-events-none disabled:opacity-40"
                        :disabled="eligibleCount === 0"
                        :title="
                            uncheckedCount
                                ? 'Remove the unchecked videos from the group, then move every eligible video to the trash, the best one included'
                                : 'Move every video of this group to the trash, the best one included'
                        "
                        @click="emit('trashAll')"
                    >
                        <Icon name="heroicons:trash" size="12" />
                        Trash all
                    </button>
                    <button
                        class="border-border text-dim rounded-md border px-3 py-1.5 text-[11px]
                            font-medium transition-all hover:bg-white/5 hover:text-white"
                        @click="emit('dismiss')"
                    >
                        Dismiss
                    </button>
                    <button
                        class="bg-lava hover:bg-lava/80 rounded-md px-3 py-1.5 text-[11px]
                            font-medium text-white transition-all disabled:opacity-50"
                        :disabled="resolveBlocker !== null"
                        :title="
                            resolveBlocker ??
                            (uncheckedCount
                                ? 'Remove the unchecked videos from the group, then keep the selected scene and trash the other eligible ones'
                                : 'Keep the selected scene and trash the others')
                        "
                        @click="emit('resolve')"
                    >
                        Resolve
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
