<script setup lang="ts">
import type { PresenceGroupKey } from '~/types/presence';

// Presence dot of a user (on an avatar): watching, online, away or offline. Nothing when the
// presence of the user is unknown or hidden.
const props = defineProps<{ userId: number; size?: 'sm' | 'lg' }>();

const presenceStore = usePresenceStore();
const view = computed(() => presenceStore.viewOf(props.userId));
const group = computed(() => (view.value ? presenceGroupOf(view.value) : null));

const colors: Record<PresenceGroupKey, string> = {
    watching: 'bg-lava shadow-[0_0_6px_rgba(255,77,77,0.7)]',
    online: 'bg-emerald shadow-[0_0_6px_rgba(16,185,129,0.6)]',
    away: 'bg-amber-400',
    offline: 'bg-white/25',
};
const label = computed(() => {
    if (!group.value) return '';
    const activity = view.value?.activity;
    if (group.value === 'away' && activity) return `Away, ${awayActivityLabel(activity)}`;
    return PRESENCE_GROUP_LABELS[group.value];
});
</script>

<template>
    <span
        v-if="group"
        class="border-void absolute rounded-full border-2"
        :class="[
            colors[group],
            props.size === 'lg'
                ? 'right-0 bottom-0 h-3.5 w-3.5'
                : '-right-0.5 -bottom-0.5 h-2.5 w-2.5',
        ]"
        :title="label"
        :aria-label="label"
        role="img"
    />
</template>
