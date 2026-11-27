<script setup lang="ts">
import type { ShareSubject } from '~/types/share';

// What the Share dialog shares, in one line (a marker shows its label and range).
const props = defineProps<{ subject: ShareSubject }>();

const line = computed(() => {
    const s = props.subject;
    switch (s.kind) {
        case 'scene':
            return { icon: 'heroicons:film', text: s.title || 'Video', extra: '' };
        case 'scenes':
            return {
                icon: 'heroicons:film',
                text: `${s.sceneIds.length} videos`,
                extra: 'one message each',
            };
        case 'marker': {
            const range =
                s.end != null && s.end > s.start
                    ? `${formatClock(s.start)} - ${formatClock(s.end)}`
                    : `from ${formatClock(s.start)}`;
            return { icon: 'heroicons:bookmark', text: s.label || 'Marker', extra: range };
        }
        case 'playlist':
            return { icon: 'heroicons:queue-list', text: s.name, extra: 'playlist' };
        case 'lobby':
            return { icon: 'heroicons:users', text: s.name, extra: 'lobby invite' };
    }
    return { icon: 'heroicons:share', text: '', extra: '' };
});
const sceneTitle = computed(() =>
    props.subject.kind === 'marker' ? (props.subject.sceneTitle ?? '') : '',
);
</script>

<template>
    <div class="border-border bg-surface/40 flex items-center gap-2 rounded-lg border px-3 py-2">
        <Icon :name="line.icon" size="14" class="text-lava shrink-0" />
        <div class="min-w-0 flex-1">
            <div class="truncate text-xs text-white">{{ line.text }}</div>
            <div v-if="sceneTitle" class="text-dim truncate text-[10px]">{{ sceneTitle }}</div>
        </div>
        <span v-if="line.extra" class="text-dim shrink-0 font-mono text-[10px]">
            {{ line.extra }}
        </span>
    </div>
</template>
