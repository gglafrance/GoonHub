<script setup lang="ts">
import type { WorkerJob } from '~/types/workers';

defineProps<{
    jobs: WorkerJob[];
}>();
</script>

<template>
    <div class="border-border mt-4 space-y-2 border-t pt-3">
        <div v-for="j in jobs" :key="j.job_id" class="flex items-center gap-3 text-[11px]">
            <span class="text-dim w-36 shrink-0">{{ phaseLabel(j.phase) }}</span>
            <NuxtLink
                :to="`/watch/${j.scene_id}`"
                class="hover:text-lava min-w-0 flex-1 truncate text-white"
            >
                {{ j.scene_title || `Scene ${j.scene_id}` }}
            </NuxtLink>
            <span class="text-dim shrink-0 font-mono text-[10px]">
                {{ j.stage || 'assigned'
                }}<template v-if="j.input_mode"> · {{ j.input_mode }}</template>
                <template v-if="j.attempt"> · try {{ j.attempt + 1 }}</template>
            </span>
            <div class="bg-border h-1.5 w-24 shrink-0 overflow-hidden rounded-full">
                <div class="bg-lava h-full transition-all" :style="{ width: `${j.progress}%` }" />
            </div>
            <span class="text-dim w-8 shrink-0 text-right font-mono text-[10px]"
                >{{ j.progress }}%</span
            >
        </div>
    </div>
</template>
