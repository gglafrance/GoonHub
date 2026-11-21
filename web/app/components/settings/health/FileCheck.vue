<script setup lang="ts">
import type { HealthCheck, LibraryHealthSummary } from '~/types/libraryHealth';

const props = defineProps<{
    summary: LibraryHealthSummary;
    task: HealthCheck | null;
    busy: boolean;
}>();

const emit = defineEmits<{
    start: [];
    cancel: [];
    refresh: [];
}>();

const lastCheck = computed(() => props.summary.last_missing_check);

const progressPct = computed(() => {
    if (!props.task || props.task.total === 0) return 0;
    return Math.min(100, (props.task.checked / props.task.total) * 100);
});

const taskLabel = computed(() =>
    props.task?.kind === 'fix_sizes' ? 'Re-reading file sizes' : 'Checking files',
);

const formatElapsed = (start: string, end?: string | null): string => {
    const ms = (end ? new Date(end) : new Date()).getTime() - new Date(start).getTime();
    const seconds = Math.max(0, Math.floor(ms / 1000));
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
    return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
};
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4 flex items-start justify-between gap-4">
            <div>
                <div class="flex items-center gap-2">
                    <Icon name="heroicons:heart" size="16" class="text-lava" />
                    <h3 class="text-sm font-semibold text-white">Library Health</h3>
                </div>
                <p class="text-dim mt-1 text-[11px]">
                    {{ summary.total_scenes.toLocaleString() }} scenes. The file check looks at
                    every scene file on disk (slow on network storage). Files are only read, never
                    modified.
                </p>
            </div>
            <div class="flex shrink-0 gap-2">
                <button
                    class="text-dim rounded-lg border border-white/10 bg-white/5 px-2 py-1.5
                        transition-all hover:bg-white/10 hover:text-white"
                    title="Refresh counts"
                    @click="emit('refresh')"
                >
                    <Icon name="heroicons:arrow-path" size="14" />
                </button>
                <button
                    v-if="!task"
                    :disabled="busy"
                    class="bg-lava hover:bg-lava-glow disabled:bg-lava/50 rounded-lg px-3 py-1.5
                        text-[11px] font-semibold text-white transition-all
                        disabled:cursor-not-allowed"
                    @click="emit('start')"
                >
                    Check Files
                </button>
                <button
                    v-else
                    class="rounded-lg border border-amber-500/30 bg-amber-500/15 px-3 py-1.5
                        text-[11px] font-semibold text-amber-500 transition-all
                        hover:bg-amber-500/25"
                    @click="emit('cancel')"
                >
                    Cancel
                </button>
            </div>
        </div>

        <!-- Running task -->
        <div v-if="task" class="border-border mb-3 rounded-lg border bg-white/2 p-4">
            <div class="mb-2 flex items-center justify-between text-xs">
                <div class="flex items-center gap-2">
                    <div class="h-2 w-2 animate-pulse rounded-full bg-blue-500" />
                    <span class="font-medium text-white">{{ taskLabel }}...</span>
                </div>
                <span class="text-dim text-[11px]">
                    {{ task.checked.toLocaleString() }} / {{ task.total.toLocaleString() }} &middot;
                    {{ formatElapsed(task.started_at) }}
                </span>
            </div>
            <div class="bg-void h-1.5 overflow-hidden rounded-full">
                <div
                    class="h-full rounded-full bg-blue-500 transition-all"
                    :style="{ width: `${progressPct}%` }"
                />
            </div>
            <div class="text-dim mt-2 flex gap-4 text-[11px]">
                <span>
                    Missing:
                    <span :class="task.missing > 0 ? 'text-amber-500' : 'text-white'">
                        {{ task.missing.toLocaleString() }}
                    </span>
                </span>
                <span>
                    Unreadable:
                    <span :class="task.unreadable > 0 ? 'text-lava' : 'text-white'">
                        {{ task.unreadable.toLocaleString() }}
                    </span>
                </span>
                <span v-if="task.kind === 'fix_sizes'">
                    Fixed: <span class="text-emerald">{{ task.fixed.toLocaleString() }}</span>
                </span>
            </div>
        </div>

        <!-- Last completed check -->
        <div class="text-dim text-[11px]">
            <template v-if="lastCheck">
                Files last checked
                <NuxtTime
                    v-if="lastCheck.completed_at"
                    :datetime="lastCheck.completed_at"
                    relative
                    class="text-white"
                />
                (<UiFormattedDate :datetime="lastCheck.completed_at" time />):
                {{ lastCheck.checked.toLocaleString() }} files in
                {{ formatElapsed(lastCheck.started_at, lastCheck.completed_at) }},
                <span :class="lastCheck.missing > 0 ? 'text-amber-500' : ''"
                    >{{ lastCheck.missing.toLocaleString() }} missing</span
                >,
                <span :class="lastCheck.unreadable > 0 ? 'text-lava' : ''"
                    >{{ lastCheck.unreadable.toLocaleString() }} unreadable</span
                >
            </template>
            <template v-else>Files never checked: missing and unreadable counts unknown.</template>
        </div>
        <div v-if="summary.last_fix_sizes" class="text-dim mt-1 text-[11px]">
            Sizes last fixed
            <NuxtTime
                v-if="summary.last_fix_sizes.completed_at"
                :datetime="summary.last_fix_sizes.completed_at"
                relative
            />
            ({{ summary.last_fix_sizes.status }}):
            {{ summary.last_fix_sizes.fixed.toLocaleString() }} updated,
            {{ summary.last_fix_sizes.unchanged.toLocaleString() }} still 0 bytes,
            {{ summary.last_fix_sizes.missing.toLocaleString() }} missing
        </div>
    </div>
</template>
