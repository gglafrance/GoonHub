<script setup lang="ts">
import type { BulkMatchResult } from '~/types/bulk-match';

defineProps<{
    failedScenes: BulkMatchResult[];
}>();

const emit = defineEmits<{
    retry: [];
    dismiss: [];
}>();
</script>

<template>
    <div class="border-border border-t bg-red-900/10 px-4 py-3">
        <div class="flex items-start justify-between">
            <div>
                <p class="text-sm font-medium text-red-400">
                    {{ failedScenes.length }} scene{{ failedScenes.length !== 1 ? 's' : '' }} failed
                </p>
                <p class="text-dim mt-1 text-xs">
                    The following scenes encountered errors during metadata application.
                </p>
            </div>
            <div class="flex gap-2">
                <button
                    @click="emit('retry')"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs font-semibold
                        text-white transition-colors"
                >
                    Retry All
                </button>
                <button
                    @click="emit('dismiss')"
                    class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                        text-xs font-medium text-white transition-all"
                >
                    Dismiss
                </button>
            </div>
        </div>

        <!-- Failed scenes list -->
        <div class="mt-3 max-h-32 overflow-y-auto">
            <div
                v-for="result in failedScenes"
                :key="result.sceneId"
                class="flex items-center gap-2 py-1"
            >
                <Icon
                    name="heroicons:exclamation-triangle"
                    size="14"
                    class="shrink-0 text-red-400"
                />
                <span class="flex-1 truncate text-xs text-white">
                    {{ result.localScene.title }}
                </span>
                <span v-if="result.error" class="text-dim truncate text-[10px]">
                    {{ result.error }}
                </span>
            </div>
        </div>
    </div>
</template>
