<script setup lang="ts">
import type { DuplicateSkippedGroup } from '~/types/duplicates';

// Groups an automatic action left untouched because not all their scenes are identical.
defineProps<{
    skipped: DuplicateSkippedGroup[];
}>();

const emit = defineEmits<{
    open: [groupId: number];
    close: [];
}>();

const { kindLabel, kindClass } = useDuplicateRelation();
</script>

<template>
    <div class="border-lava/20 bg-lava/5 mb-3 rounded-md border px-3 py-2 text-[11px]">
        <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-1.5 text-white">
                <Icon name="heroicons:shield-exclamation" size="14" class="text-lava" />
                {{ skipped.length }} group(s) skipped: not all their scenes are identical. Review
                and resolve them by hand.
            </div>
            <button class="text-dim hover:text-white" @click="emit('close')">
                <Icon name="heroicons:x-mark" size="12" />
            </button>
        </div>
        <ul class="mt-1.5 max-h-40 space-y-1 overflow-y-auto">
            <li v-for="s in skipped" :key="s.group_id" class="flex items-start gap-2">
                <button
                    class="text-lava/80 hover:text-lava shrink-0 font-mono transition-colors"
                    title="Open this group"
                    @click="emit('open', s.group_id)"
                >
                    #{{ s.group_id }}
                </button>
                <span
                    class="shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-semibold"
                    :class="kindClass(s.kind)"
                >
                    {{ kindLabel(s.kind) }}
                </span>
                <span class="text-dim">{{ s.reason }}</span>
            </li>
        </ul>
    </div>
</template>
