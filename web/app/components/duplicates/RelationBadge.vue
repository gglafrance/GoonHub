<script setup lang="ts">
import type { DuplicateGroupMember } from '~/types/duplicates';

// How a group member relates to the scene it was matched against.
defineProps<{
    member: DuplicateGroupMember;
}>();

const { memberRelationLabel, memberRelationTitle, relationClass } = useDuplicateRelation();
</script>

<template>
    <span
        class="inline-flex items-center rounded-full px-1.5 py-0.5 text-[9px] font-semibold
            whitespace-nowrap"
        :class="relationClass(member.relation)"
        :title="memberRelationTitle(member)"
    >
        {{ memberRelationLabel(member) }}
    </span>
</template>
