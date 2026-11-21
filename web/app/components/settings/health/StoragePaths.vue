<script setup lang="ts">
import type { StoragePathHealth } from '~/types/libraryHealth';

defineProps<{
    paths: StoragePathHealth[];
}>();

const { formatSize } = useFormatter();
</script>

<template>
    <div class="glass-panel p-5">
        <div class="mb-4">
            <h3 class="text-sm font-semibold text-white">Storage Paths</h3>
            <p class="text-dim mt-1 text-[11px]">
                Scenes and their total size as recorded in the database, next to the usage of the
                filesystem holding each path.
            </p>
        </div>

        <div v-if="paths.length === 0" class="text-dim py-4 text-center text-xs">
            No storage paths configured
        </div>
        <div v-else class="space-y-3">
            <div
                v-for="path in paths"
                :key="path.id ?? 'none'"
                class="border-border rounded-lg border bg-white/2 p-3"
            >
                <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                        <div class="text-xs font-medium text-white">{{ path.name }}</div>
                        <code
                            v-if="path.path"
                            class="text-dim bg-void/50 mt-1 inline-block max-w-full truncate rounded
                                px-1.5 py-0.5 text-[11px]"
                        >
                            {{ path.path }}
                        </code>
                        <p v-else class="text-dim mt-1 text-[11px]">
                            Scenes not attached to any configured storage path
                        </p>
                    </div>
                    <div class="shrink-0 text-right text-[11px]">
                        <div class="text-white tabular-nums">
                            {{ path.scene_count.toLocaleString() }} scenes
                        </div>
                        <div class="text-dim tabular-nums">
                            {{ formatSize(path.total_size) }} in DB
                        </div>
                    </div>
                </div>

                <div v-if="path.disk_usage" class="mt-2.5 flex items-center gap-3">
                    <div class="bg-void h-1.5 flex-1 overflow-hidden rounded-full">
                        <div
                            class="h-full rounded-full transition-all"
                            :class="path.disk_usage.used_pct > 90 ? 'bg-lava' : 'bg-emerald'"
                            :style="{ width: `${Math.min(path.disk_usage.used_pct, 100)}%` }"
                        />
                    </div>
                    <span class="text-dim text-[10px] whitespace-nowrap tabular-nums">
                        {{ formatSize(path.disk_usage.free_bytes) }} free of
                        {{ formatSize(path.disk_usage.total_bytes) }} ({{
                            path.disk_usage.used_pct.toFixed(1)
                        }}% used)
                    </span>
                </div>
                <p v-else-if="path.id !== null" class="text-lava/80 mt-2 text-[10px]">
                    Disk usage unavailable: the path could not be read
                </p>
            </div>
        </div>
    </div>
</template>
