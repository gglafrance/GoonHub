<script setup lang="ts">
// Selection and bulk actions of the copy list.
defineProps<{
    selectedCount: number;
    // Size of the selected copies loaded on the client, and how many are not loaded
    selectedSize: number;
    unknownCount: number;
    pageCount: number;
    matchingCount: number;
    busy: boolean;
}>();

const emit = defineEmits<{
    selectPage: [];
    selectMatching: [];
    clear: [];
    forget: [];
    delete: [];
}>();

const { formatSize } = useFormatter();
</script>

<template>
    <div
        class="border-border bg-panel/80 sticky top-[var(--app-header-h)] z-30 mb-3 flex flex-wrap
            items-center justify-between gap-2 rounded-lg border px-3 py-2 backdrop-blur-md"
    >
        <div class="flex flex-wrap items-center gap-3 text-[11px]">
            <span class="text-white">
                <span class="text-lava font-semibold">{{ selectedCount }}</span>
                selected<span v-if="selectedCount" class="text-dim">
                    · {{ formatSize(selectedSize)
                    }}{{ unknownCount ? ` + ${unknownCount} not loaded` : '' }}</span
                >
            </span>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="pageCount === 0"
                @click="emit('selectPage')"
            >
                Select this page ({{ pageCount }})
            </button>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="busy || matchingCount === 0"
                title="Every copy matching the filters, on every page (up to the bulk limit)"
                @click="emit('selectMatching')"
            >
                Select all matching ({{ matchingCount }})
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
                class="border-border text-dim rounded-md border px-2.5 py-1 text-[11px] font-medium
                    transition-all hover:border-white/30 hover:text-white disabled:opacity-40"
                :disabled="busy || selectedCount === 0"
                title="Remove the selected copies from the list; the files stay on disk"
                @click="emit('forget')"
            >
                Forget
            </button>
            <button
                class="border-lava/40 text-lava hover:bg-lava/10 rounded-md border px-2.5 py-1
                    text-[11px] font-medium transition-all disabled:opacity-40"
                :disabled="busy || selectedCount === 0"
                @click="emit('delete')"
            >
                Delete files
            </button>
        </div>
    </div>
</template>
