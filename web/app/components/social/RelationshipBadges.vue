<script setup lang="ts">
import type { Relationship } from '~/types/social';

const props = defineProps<{ relationship: Relationship }>();

const badges = computed(() => relationshipBadges(props.relationship));

const badgeClass: Record<string, string> = {
    friend: 'border-emerald/30 bg-emerald/10 text-emerald',
    mutual: 'border-lava/30 bg-lava/10 text-lava',
    follows_you: 'border-border bg-white/5 text-dim',
    request_received: 'border-lava/30 bg-lava/10 text-lava',
    request_sent: 'border-border bg-white/5 text-dim',
};
</script>

<template>
    <div v-if="badges.length" class="flex flex-wrap items-center gap-1">
        <span
            v-for="b in badges"
            :key="b.key"
            class="rounded-full border px-1.5 py-px font-mono text-[9px] tracking-wider uppercase"
            :class="badgeClass[b.key]"
        >
            {{ b.label }}
        </span>
    </div>
</template>
