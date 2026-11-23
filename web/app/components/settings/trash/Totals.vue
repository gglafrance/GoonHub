<script setup lang="ts">
// Header strip of the trash: size of the whole trash, of the current filter, retention.
import type { TrashTotals } from '~/types/trash';

defineProps<{
    all: TrashTotals;
    filtered: TrashTotals;
    filterActive: boolean;
    retentionDays: number;
}>();

const emit = defineEmits<{ emptyTrash: [] }>();

const { formatSize } = useFormatter();
</script>

<template>
    <div class="flex flex-wrap items-stretch gap-2">
        <div class="border-border min-w-36 rounded-lg border bg-white/2 px-3 py-2">
            <div class="text-dim text-[10px] tracking-wide uppercase">Whole trash</div>
            <div class="mt-0.5 text-sm font-semibold text-white">
                {{ formatSize(all.size) }}
                <span class="text-dim text-[11px] font-normal">
                    · {{ all.count.toLocaleString() }} video{{ all.count === 1 ? '' : 's' }}
                </span>
            </div>
        </div>
        <div
            v-if="filterActive"
            class="border-lava/30 bg-lava/5 min-w-36 rounded-lg border px-3 py-2"
        >
            <div class="text-lava/80 text-[10px] tracking-wide uppercase">Current filter</div>
            <div class="mt-0.5 text-sm font-semibold text-white">
                {{ formatSize(filtered.size) }}
                <span class="text-dim text-[11px] font-normal">
                    · {{ filtered.count.toLocaleString() }} video{{
                        filtered.count === 1 ? '' : 's'
                    }}
                </span>
            </div>
        </div>
        <div class="border-border flex-1 rounded-lg border bg-white/2 px-3 py-2">
            <div class="text-dim text-[10px] tracking-wide uppercase">Retention</div>
            <div class="text-dim mt-0.5 text-[11px]">
                Purged {{ retentionDays }} day{{ retentionDays === 1 ? '' : 's' }} after being
                trashed (sooner for resolved duplicates with a scheduled deletion). Files on
                read-only storage paths are never deleted.
            </div>
        </div>
        <button
            v-if="all.count > 0"
            class="self-center rounded-lg bg-red-600 px-3 py-1.5 text-[11px] font-semibold
                text-white transition-all hover:bg-red-500"
            @click="emit('emptyTrash')"
        >
            Empty Trash
        </button>
    </div>
</template>
