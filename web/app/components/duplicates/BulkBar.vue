<script setup lang="ts">
// Bulk actions on the selected duplicate groups.
defineProps<{
    selectedCount: number;
    selectableCount: number;
    // a kind filter is active: "select all" takes the groups of that kind only
    kindFiltered: boolean;
    busy: boolean;
    deletionEnabled: boolean;
    deletionDelayHours: number;
}>();

const emit = defineEmits<{
    selectAll: [];
    selectAllGroups: [];
    clear: [];
    autoSelect: [];
    autoResolve: [];
    trashAll: [];
}>();
</script>

<template>
    <div
        class="border-border bg-panel/80 sticky top-[var(--app-header-h)] z-30 mb-3 flex flex-wrap
            items-center justify-between gap-2 rounded-lg border px-3 py-2 backdrop-blur-md"
    >
        <div class="flex items-center gap-3 text-[11px]">
            <span class="text-white">
                <span class="text-lava font-semibold">{{ selectedCount }}</span>
                selected
            </span>
            <button
                class="text-dim transition-colors hover:text-white"
                :disabled="selectableCount === 0"
                @click="emit('selectAll')"
            >
                Select all unresolved on this page ({{ selectableCount }})
            </button>
            <button
                class="text-dim transition-colors hover:text-white disabled:opacity-40"
                :disabled="busy"
                title="Every unresolved group, on every page (up to the bulk limit)"
                @click="emit('selectAllGroups')"
            >
                Select all unresolved groups{{ kindFiltered ? ' of this kind' : '' }}
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
            <span
                v-if="deletionEnabled"
                class="text-lava/80 text-[10px]"
                title="Video files on read-only storage paths are kept"
            >
                Files not kept are deleted
                {{ deletionDelayHours === 0 ? 'right away' : `after ${deletionDelayHours}h` }}
            </span>
            <button
                class="border-border text-dim hover:border-emerald/40 hover:text-emerald flex
                    items-center gap-1 rounded-md border px-2.5 py-1 text-[11px] font-medium
                    transition-all disabled:opacity-40"
                :disabled="selectedCount === 0 || busy"
                @click="emit('autoSelect')"
            >
                <Icon name="heroicons:sparkles" size="12" />
                Auto-select best
            </button>
            <button
                class="border-border text-dim flex items-center gap-1 rounded-md border px-2.5 py-1
                    text-[11px] font-medium transition-all hover:border-red-500/40
                    hover:text-red-400 disabled:opacity-40"
                :disabled="selectedCount === 0 || busy"
                title="Move every video of the selected groups to the trash, the best ones included"
                @click="emit('trashAll')"
            >
                <Icon name="heroicons:trash" size="12" />
                Trash all videos
            </button>
            <button
                class="bg-lava hover:bg-lava/80 flex items-center gap-1 rounded-md px-2.5 py-1
                    text-[11px] font-medium text-white transition-all disabled:opacity-40"
                :disabled="selectedCount === 0 || busy"
                @click="emit('autoResolve')"
            >
                <Icon v-if="busy" name="svg-spinners:ring-resize" size="12" />
                <Icon v-else name="heroicons:bolt" size="12" />
                Auto-resolve selected
            </button>
        </div>
    </div>
</template>
