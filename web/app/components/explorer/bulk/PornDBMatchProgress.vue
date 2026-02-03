<script setup lang="ts">
import type { ApplyPhase } from '~/types/bulk-match';

const props = defineProps<{
    phase: ApplyPhase;
    progress: { current: number; total: number; failed: number };
}>();

const progressPercent = computed(() => {
    if (props.progress.total === 0) return 0;
    return Math.round((props.progress.current / props.progress.total) * 100);
});
</script>

<template>
    <div class="border-border border-t bg-black/30 px-4 py-3">
        <div class="flex items-center gap-4">
            <!-- Progress bar -->
            <div class="flex-1">
                <div class="bg-void h-2 overflow-hidden rounded-full">
                    <div
                        class="bg-lava h-full rounded-full transition-all duration-300"
                        :style="{ width: `${progressPercent}%` }"
                    />
                </div>
            </div>

            <!-- Stats -->
            <div class="flex shrink-0 items-center gap-3 text-xs">
                <span class="text-white"> {{ progress.current }} / {{ progress.total }} </span>
                <span v-if="progress.failed > 0" class="text-red-400">
                    {{ progress.failed }} failed
                </span>
            </div>
        </div>

        <!-- Status text -->
        <p class="text-dim mt-2 text-xs">
            <template v-if="phase === 'applying'">
                Applying metadata to scene {{ progress.current }} of {{ progress.total }}...
            </template>
            <template v-else-if="phase === 'done'">
                Done! {{ progress.total - progress.failed }} scenes updated successfully.
            </template>
        </p>
    </div>
</template>
