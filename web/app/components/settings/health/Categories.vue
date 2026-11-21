<script setup lang="ts">
import type { HealthAction, HealthCategory, LibraryHealthSummary } from '~/types/libraryHealth';
import type { HealthCategoryInfo } from '~/utils/libraryHealth';

const props = defineProps<{
    summary: LibraryHealthSummary;
    selected: HealthCategory | null;
    taskRunning: boolean;
    busy: boolean;
}>();

const emit = defineEmits<{
    view: [category: HealthCategory];
    action: [action: HealthAction];
}>();

const countOf = (category: HealthCategoryInfo): number | undefined =>
    props.summary.counts[category.key];

const percentOf = (count: number): string => {
    if (!props.summary.total_scenes || count === 0) return '';
    const pct = (count / props.summary.total_scenes) * 100;
    return pct < 0.1 ? '<0.1%' : `${pct.toFixed(1)}%`;
};

// Why an action cannot run right now (empty = it can)
const actionBlocker = (category: HealthCategoryInfo): string => {
    if (category.action === 'fingerprint' && !props.summary.fingerprint_enabled) {
        return 'Duplicate detection is disabled';
    }
    if (category.action === 'fix_sizes' && props.taskRunning) {
        return 'A file task is already running';
    }
    return '';
};

const note = (category: HealthCategoryInfo): string => {
    if (category.key === 'no_preview' && !props.summary.scene_preview_enabled) {
        return 'Scene previews are disabled in Jobs > Processing';
    }
    if (category.fromCheck && countOf(category) === undefined) {
        return 'Run a file check to count them';
    }
    return '';
};
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4">
            <h3 class="text-sm font-semibold text-white">Issues</h3>
            <p class="text-dim mt-1 text-[11px]">
                Counted live from the database, except missing and unreadable files which come from
                the last file check. Actions only queue the jobs that are missing; you are told how
                many before anything is queued.
            </p>
        </div>

        <div class="space-y-5">
            <div v-for="group in HEALTH_GROUPS" :key="group.id">
                <div
                    class="text-dim mb-2 flex items-center gap-1.5 text-[10px] font-semibold
                        tracking-wider uppercase"
                >
                    <Icon :name="group.icon" size="12" />
                    {{ group.label }}
                </div>
                <div class="border-border divide-border/50 divide-y rounded-lg border bg-white/2">
                    <div
                        v-for="category in group.categories"
                        :key="category.key"
                        class="flex items-center gap-3 px-3 py-2.5 transition-colors"
                        :class="selected === category.key ? 'bg-lava/5' : ''"
                    >
                        <div class="min-w-0 flex-1">
                            <div class="flex items-center gap-2">
                                <span class="text-xs font-medium text-white">
                                    {{ category.label }}
                                </span>
                            </div>
                            <p class="text-dim mt-0.5 truncate text-[11px]">
                                {{ category.description }}
                            </p>
                            <p v-if="note(category)" class="mt-0.5 text-[10px] text-amber-500/80">
                                {{ note(category) }}
                            </p>
                        </div>

                        <div class="w-24 shrink-0 text-right">
                            <template v-if="countOf(category) !== undefined">
                                <span
                                    class="text-sm font-semibold tabular-nums"
                                    :class="
                                        countOf(category) === 0 ? 'text-emerald' : 'text-amber-500'
                                    "
                                >
                                    {{ countOf(category)!.toLocaleString() }}
                                </span>
                                <span
                                    v-if="percentOf(countOf(category)!)"
                                    class="text-dim block text-[10px]"
                                >
                                    {{ percentOf(countOf(category)!) }}
                                </span>
                            </template>
                            <span v-else class="text-dim text-sm">&mdash;</span>
                        </div>

                        <div class="flex w-40 shrink-0 justify-end gap-2">
                            <button
                                v-if="category.action && (countOf(category) ?? 0) > 0"
                                :disabled="busy || actionBlocker(category) !== ''"
                                :title="actionBlocker(category)"
                                class="bg-lava/80 hover:bg-lava disabled:bg-lava/30 rounded-lg
                                    px-2.5 py-1 text-[11px] font-medium whitespace-nowrap text-white
                                    transition-all disabled:cursor-not-allowed"
                                @click="emit('action', category.action)"
                            >
                                {{ healthActionLabel(category.action) }}
                            </button>
                            <button
                                :disabled="!countOf(category)"
                                class="rounded-lg border border-white/10 bg-white/5 px-2.5 py-1
                                    text-[11px] font-medium text-white transition-all
                                    hover:bg-white/10 disabled:cursor-not-allowed
                                    disabled:opacity-30"
                                @click="emit('view', category.key)"
                            >
                                {{ selected === category.key ? 'Hide' : 'View' }}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
