<script setup lang="ts">
// Floating bar of the unsaved layout changes.
defineProps<{ saving: boolean }>();
defineEmits<{ save: []; discard: [] }>();
</script>

<template>
    <div
        class="border-border bg-panel/95 sticky bottom-4 z-10 flex items-center justify-between
            gap-3 rounded-lg border px-4 py-2.5 shadow-2xl backdrop-blur-md"
    >
        <div class="text-lava flex items-center gap-1.5 text-xs font-medium">
            <span class="bg-lava h-1.5 w-1.5 animate-pulse rounded-full" />
            Unsaved changes
        </div>
        <div class="flex items-center gap-2">
            <button
                type="button"
                class="text-dim rounded-lg px-3 py-1.5 text-[11px] transition-colors
                    hover:text-white/80"
                :disabled="saving"
                @click="$emit('discard')"
            >
                Discard
            </button>
            <button
                type="button"
                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                :disabled="saving"
                @click="$emit('save')"
            >
                {{ saving ? 'Saving...' : 'Save' }}
            </button>
        </div>
    </div>
</template>
