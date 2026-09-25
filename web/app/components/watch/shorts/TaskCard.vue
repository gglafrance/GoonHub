<script setup lang="ts">
import type { ShortTask } from '~/types/shorts';
import { formatShortClock, formatShortSpan } from '~/composables/useShortEditor';

// A short still being encoded, or one that failed.
const props = defineProps<{
    task: ShortTask;
}>();

const emit = defineEmits<{
    dismiss: [taskId: string];
}>();

const failed = computed(() => props.task.status === 'failed');
</script>

<template>
    <div
        class="bg-surface overflow-hidden rounded-lg border"
        :class="failed ? 'border-lava/40' : 'border-border'"
    >
        <div
            class="relative flex aspect-video flex-col items-center justify-center gap-2 bg-black/40
                p-3 text-center"
        >
            <template v-if="failed">
                <Icon name="heroicons:exclamation-triangle" size="22" class="text-lava" />
                <span class="text-lava text-[11px] font-medium">Short failed</span>
                <span class="text-dim line-clamp-3 text-[10px] break-all">{{ task.error }}</span>
            </template>
            <template v-else>
                <Icon name="svg-spinners:ring-resize" size="20" class="text-lava" />
                <span class="text-dim text-[11px]">
                    {{ task.status === 'queued' ? 'Waiting to encode' : 'Encoding' }}
                    <span v-if="task.status !== 'queued'" class="font-mono text-white"
                        >{{ task.percent }}%</span
                    >
                </span>
                <div class="h-1 w-3/4 overflow-hidden rounded-full bg-white/10">
                    <div
                        class="bg-lava h-full rounded-full transition-all duration-500"
                        :style="{ width: `${task.percent}%` }"
                    />
                </div>
            </template>
        </div>
        <div class="flex items-center gap-2 px-2 py-1.5">
            <span class="min-w-0 flex-1 truncate text-[11px] text-white" :title="task.title">
                {{ task.title || 'New short' }}
            </span>
            <span class="text-dim shrink-0 font-mono text-[10px]">
                {{ formatShortClock(task.start) }} · {{ formatShortSpan(task.end - task.start) }}
            </span>
            <button
                v-if="failed"
                class="text-dim shrink-0 text-[10px] hover:text-white"
                @click="emit('dismiss', task.task_id)"
            >
                Dismiss
            </button>
        </div>
    </div>
</template>
