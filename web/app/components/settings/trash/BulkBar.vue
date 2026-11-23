<script setup lang="ts">
// Selection and bulk actions of the trash list.
defineProps<{
    selectedCount: number;
    selectedSize: number;
    pageCount: number;
    resultCount: number;
    busy: boolean;
}>();

const emit = defineEmits<{
    selectPage: [];
    selectAll: [];
    clear: [];
    restore: [];
    delete: [];
}>();

const { formatSize } = useFormatter();
</script>

<template>
    <div
        class="border-border bg-panel/80 sticky top-14 z-30 flex flex-wrap items-center
            justify-between gap-2 rounded-lg border px-3 py-2 backdrop-blur-md"
    >
        <div class="flex flex-wrap items-center gap-3 text-[11px]">
            <span class="text-white">
                <span class="text-lava font-semibold">{{ selectedCount }}</span>
                selected
                <span v-if="selectedCount" class="text-dim">({{ formatSize(selectedSize) }})</span>
            </span>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="pageCount === 0"
                @click="emit('selectPage')"
            >
                Select page ({{ pageCount }})
            </button>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="busy || resultCount === 0"
                :title="`Every result of the current filter, on every page (up to ${TRASH_BULK_LIMIT})`"
                @click="emit('selectAll')"
            >
                Select all results ({{ Math.min(resultCount, TRASH_BULK_LIMIT).toLocaleString() }})
            </button>
            <button
                v-if="selectedCount > 0"
                class="text-dim transition-colors hover:text-white"
                @click="emit('clear')"
            >
                Clear
            </button>
        </div>
        <div class="flex items-center gap-2">
            <button
                class="border-border text-dim hover:border-emerald/40 hover:text-emerald flex
                    items-center gap-1 rounded-md border px-2.5 py-1 text-[11px] font-medium
                    transition-all disabled:opacity-40"
                :disabled="selectedCount === 0 || busy"
                @click="emit('restore')"
            >
                <Icon name="heroicons:arrow-uturn-left" size="12" />
                Restore selected
            </button>
            <button
                class="flex items-center gap-1 rounded-md bg-red-600 px-2.5 py-1 text-[11px]
                    font-medium text-white transition-all hover:bg-red-500 disabled:opacity-40"
                :disabled="selectedCount === 0 || busy"
                @click="emit('delete')"
            >
                <Icon v-if="busy" name="svg-spinners:ring-resize" size="12" />
                <Icon v-else name="heroicons:trash" size="12" />
                Delete permanently
            </button>
        </div>
    </div>
</template>
